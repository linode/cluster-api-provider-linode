/*
Copyright 2023 Akamai Technologies, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"context"
	b64 "encoding/base64"
	"encoding/gob"
	"errors"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
	"github.com/linode/linodego"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	infrav1alpha1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/cluster-api-provider-linode/util"
)

// Size limit in bytes on the decoded metadata.user_data for cloud-init
// The decoded user_data must not exceed 16384 bytes per the Linode API
const maxBootstrapDataBytes = 16384

func (*LinodeMachineReconciler) newCreateConfig(ctx context.Context, machineScope *scope.MachineScope, tags []string, logger logr.Logger) (*linodego.InstanceCreateOptions, error) {
	var err error

	createConfig := linodeMachineSpecToInstanceCreateConfig(machineScope.LinodeMachine.Spec)
	if createConfig == nil {
		err = errors.New("failed to convert machine spec to create instance config")

		logger.Error(err, "Panic! Struct of LinodeMachineSpec is different than InstanceCreateOptions")

		return nil, err
	}

	createConfig.Booted = util.Pointer(false)

	createConfig.PrivateIP = true

	bootstrapData, err := machineScope.GetBootstrapData(ctx)
	if err != nil {
		logger.Info("Failed to get bootstrap data", "error", err.Error())

		return nil, err
	}
	if len(bootstrapData) > maxBootstrapDataBytes {
		err = errors.New("bootstrap data too large")
		logger.Info(fmt.Sprintf("decoded bootstrap data exceeds size limit of %d bytes", maxBootstrapDataBytes), "error", err.Error())

		return nil, err
	}
	createConfig.Metadata = &linodego.InstanceMetadataOptions{
		UserData: b64.StdEncoding.EncodeToString(bootstrapData),
	}

	if createConfig.Tags == nil {
		createConfig.Tags = []string{}
	}
	createConfig.Tags = append(createConfig.Tags, tags...)

	if createConfig.Label == "" {
		createConfig.Label = machineScope.LinodeMachine.Name
	}

	if createConfig.Image == "" {
		createConfig.Image = reconciler.DefaultMachineControllerLinodeImage
	}
	if createConfig.RootPass == "" {
		createConfig.RootPass = uuid.NewString()
	}

	return createConfig, nil
}

func (r *LinodeMachineReconciler) linodeClusterToLinodeMachines(logger logr.Logger) handler.MapFunc {
	logger = logger.WithName("LinodeMachineReconciler").WithName("linodeClusterToLinodeMachines")

	return func(ctx context.Context, o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		linodeCluster, ok := o.(*infrav1alpha1.LinodeCluster)
		if !ok {
			logger.Info("Failed to cast object to Cluster")

			return nil
		}

		if !linodeCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			logger.Info("Cluster has a deletion timestamp, skipping mapping")

			return nil
		}

		cluster, err := kutil.GetOwnerCluster(ctx, r.Client, linodeCluster.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			logger.Info("Cluster for LinodeCluster not found, skipping mapping")

			return nil
		case err != nil:
			logger.Error(err, "Failed to get owning cluster, skipping mapping")

			return nil
		}

		request, err := r.requestsForCluster(ctx, cluster.Namespace, cluster.Name)
		if err != nil {
			logger.Error(err, "Failed to create request for cluster")

			return nil
		}

		return request
	}
}

func (r *LinodeMachineReconciler) requeueLinodeMachinesForUnpausedCluster(logger logr.Logger) handler.MapFunc {
	logger = logger.WithName("LinodeMachineReconciler").WithName("requeueLinodeMachinesForUnpausedCluster")

	return func(ctx context.Context, o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		cluster, ok := o.(*clusterv1.Cluster)
		if !ok {
			logger.Info("Failed to cast object to Cluster")

			return nil
		}

		if !cluster.ObjectMeta.DeletionTimestamp.IsZero() {
			logger.Info("Cluster has a deletion timestamp, skipping mapping")

			return nil
		}

		requests, err := r.requestsForCluster(ctx, cluster.Namespace, cluster.Name)
		if err != nil {
			logger.Error(err, "Failed to create request for cluster")

			return nil
		}

		return requests
	}
}

func (r *LinodeMachineReconciler) requestsForCluster(ctx context.Context, namespace, name string) ([]ctrl.Request, error) {
	labels := map[string]string{clusterv1.ClusterNameLabel: name}

	machineList := clusterv1.MachineList{}
	if err := r.Client.List(ctx, &machineList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	result := make([]ctrl.Request, 0, len(machineList.Items))
	for _, item := range machineList.Items {
		if item.Spec.InfrastructureRef.GroupVersionKind().Kind != "LinodeMachine" || item.Spec.InfrastructureRef.Name == "" {
			continue
		}

		infraNs := item.Spec.InfrastructureRef.Namespace
		if infraNs == "" {
			infraNs = item.Namespace
		}

		result = append(result, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: infraNs,
				Name:      item.Spec.InfrastructureRef.Name,
			},
		})
	}

	return result, nil
}

func (r *LinodeMachineReconciler) getVPCInterfaceConfig(ctx context.Context, machineScope *scope.MachineScope, existingIfaces []linodego.InstanceConfigInterfaceCreateOptions, logger logr.Logger) (*linodego.InstanceConfigInterfaceCreateOptions, error) {
	name := machineScope.LinodeCluster.Spec.VPCRef.Name
	namespace := machineScope.LinodeCluster.Spec.VPCRef.Namespace
	if namespace == "" {
		namespace = machineScope.LinodeCluster.Namespace
	}

	logger = logger.WithValues("vpcName", name, "vpcNamespace", namespace)

	linodeVPC := infrav1alpha1.LinodeVPC{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	if err := r.Get(ctx, client.ObjectKeyFromObject(&linodeVPC), &linodeVPC); err != nil {
		logger.Error(err, "Failed to fetch LinodeVPC")

		return nil, err
	} else if !linodeVPC.Status.Ready || linodeVPC.Spec.VPCID == nil {
		logger.Info("LinodeVPC is not available")

		return nil, errors.New("vpc is not available")
	}

	hasPrimary := false
	for i := range existingIfaces {
		if existingIfaces[i].Primary {
			hasPrimary = true

			break
		}
	}

	var subnetID int
	vpc, err := machineScope.LinodeClient.GetVPC(ctx, *linodeVPC.Spec.VPCID)
	switch {
	case err != nil:
		logger.Error(err, "Failed to fetch LinodeVPC")

		return nil, err
	case vpc == nil:
		err = errors.New("failed to fetch VPC")

		logger.Error(err, "Failed to fetch VPC")

		return nil, err
	case len(vpc.Subnets) == 0:
		err = errors.New("failed to find subnet")

		logger.Error(err, "Failed to find subnet")

		return nil, err
	default:
		// Place node into the least busy subnet
		sort.Slice(vpc.Subnets, func(i, j int) bool {
			return len(vpc.Subnets[i].Linodes) > len(vpc.Subnets[j].Linodes)
		})

		subnetID = vpc.Subnets[0].ID
	}

	return &linodego.InstanceConfigInterfaceCreateOptions{
		Purpose:  linodego.InterfacePurposeVPC,
		Primary:  !hasPrimary,
		SubnetID: &subnetID,
	}, nil
}

func linodeMachineSpecToInstanceCreateConfig(machineSpec infrav1alpha1.LinodeMachineSpec) *linodego.InstanceCreateOptions {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(machineSpec)
	if err != nil {
		return nil
	}

	var createConfig linodego.InstanceCreateOptions
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&createConfig)
	if err != nil {
		return nil
	}

	return &createConfig
}
