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
	"slices"
	"sort"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/linode/linodego"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	kutil "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"
	"github.com/linode/cluster-api-provider-linode/cloud/scope"
	"github.com/linode/cluster-api-provider-linode/cloud/services"
	"github.com/linode/cluster-api-provider-linode/util"
	"github.com/linode/cluster-api-provider-linode/util/reconciler"
)

// Size limit in bytes on the decoded metadata.user_data for cloud-init
// The decoded user_data must not exceed 16384 bytes per the Linode API
const maxBootstrapDataBytes = 16384

var (
	errNoPublicIPv4Addrs      = errors.New("no public ipv4 addresses set")
	errNoPublicIPv6Addrs      = errors.New("no public IPv6 address set")
	errNoPublicIPv6SLAACAddrs = errors.New("no public SLAAC address set")
)

func (r *LinodeMachineReconciler) newCreateConfig(ctx context.Context, machineScope *scope.MachineScope, tags []string, logger logr.Logger) (*linodego.InstanceCreateOptions, error) {
	var err error

	createConfig := linodeMachineSpecToInstanceCreateConfig(machineScope.LinodeMachine.Spec)
	if createConfig == nil {
		err = errors.New("failed to convert machine spec to create instance config")

		logger.Error(err, "Panic! Struct of LinodeMachineSpec is different than InstanceCreateOptions")

		return nil, err
	}

	createConfig.Booted = util.Pointer(false)

	if err := setUserData(ctx, machineScope, createConfig, logger); err != nil {
		return nil, err
	}

	if machineScope.LinodeMachine.Spec.PrivateIP != nil {
		createConfig.PrivateIP = *machineScope.LinodeMachine.Spec.PrivateIP
	} else {
		createConfig.PrivateIP = true
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

	// if vpc, attach additional interface as eth0 to linode
	if machineScope.LinodeCluster.Spec.VPCRef != nil {
		iface, err := r.getVPCInterfaceConfig(ctx, machineScope, logger)
		if err != nil {
			logger.Error(err, "Failed to get VPC interface config")

			return nil, err
		}

		// add VPC interface as first interface
		createConfig.Interfaces = slices.Insert(createConfig.Interfaces, 0, *iface)
	}

	if machineScope.LinodeMachine.Spec.PlacementGroupRef != nil {
		pgID, err := r.getPlacementGroupID(ctx, machineScope, logger)
		if err != nil {
			logger.Error(err, "Failed to get Placement Group config")
			return nil, err
		}
		createConfig.PlacementGroup = &linodego.InstanceCreatePlacementGroupOptions{
			ID: pgID,
		}
	}

	return createConfig, nil
}

func (r *LinodeMachineReconciler) buildInstanceAddrs(ctx context.Context, machineScope *scope.MachineScope, instanceID int) ([]clusterv1.MachineAddress, error) {
	addresses, err := machineScope.LinodeClient.GetInstanceIPAddresses(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get instance ips: %w", err)
	}

	// get the default instance config
	configs, err := machineScope.LinodeClient.ListInstanceConfigs(ctx, instanceID, &linodego.ListOptions{})
	if err != nil || len(configs) == 0 {
		return nil, fmt.Errorf("list instance configs: %w", err)
	}

	ips := []clusterv1.MachineAddress{}
	// check if a node has public ipv4 ip and store it
	if len(addresses.IPv4.Public) == 0 {
		return nil, errNoPublicIPv4Addrs
	}
	ips = append(ips, clusterv1.MachineAddress{
		Address: addresses.IPv4.Public[0].Address,
		Type:    clusterv1.MachineExternalIP,
	})

	// check if a node has public ipv6 ip and store it
	if addresses.IPv6 == nil {
		return nil, errNoPublicIPv6Addrs
	}
	if addresses.IPv6.SLAAC == nil {
		return nil, errNoPublicIPv6SLAACAddrs
	}
	ips = append(ips, clusterv1.MachineAddress{
		Address: addresses.IPv6.SLAAC.Address,
		Type:    clusterv1.MachineExternalIP,
	})

	// Iterate over interfaces in config and find VPC specific ips
	for _, iface := range configs[0].Interfaces {
		if iface.VPCID != nil && iface.IPv4.VPC != "" {
			ips = append(ips, clusterv1.MachineAddress{
				Address: iface.IPv4.VPC,
				Type:    clusterv1.MachineInternalIP,
			})
		}
	}

	// if a node has private ip, store it as well
	// NOTE: We specifically store VPC ips first so that they are used first during
	//       bootstrap when we set `registrationMethod: internal-only-ips`
	if len(addresses.IPv4.Private) != 0 {
		ips = append(ips, clusterv1.MachineAddress{
			Address: addresses.IPv4.Private[0].Address,
			Type:    clusterv1.MachineInternalIP,
		})
	}

	return ips, nil
}

func (r *LinodeMachineReconciler) getOwnerMachine(ctx context.Context, linodeMachine infrav1alpha2.LinodeMachine, log logr.Logger) (*clusterv1.Machine, error) {
	machine, err := kutil.GetOwnerMachine(ctx, r.TracedClient(), linodeMachine.ObjectMeta)
	if err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch owner machine")
		}

		return nil, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef, skipping reconciliation")

		return nil, err
	}
	if skippedMachinePhases[machine.Status.Phase] {
		return nil, err
	}
	match := false
	for i := range linodeMachine.OwnerReferences {
		if match = linodeMachine.OwnerReferences[i].UID == machine.UID; match {
			break
		}
	}
	if !match {
		log.Info("Failed to find the referenced owner machine, skipping reconciliation", "references", linodeMachine.OwnerReferences, "machine", machine.ObjectMeta)

		return nil, err
	}

	return machine, nil
}

func (r *LinodeMachineReconciler) getClusterFromMetadata(ctx context.Context, machine clusterv1.Machine, log logr.Logger) (*clusterv1.Cluster, error) {
	cluster, err := kutil.GetClusterFromMetadata(ctx, r.TracedClient(), machine.ObjectMeta)
	if err != nil {
		if err = client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Failed to fetch cluster by label")
		}

		return nil, err
	}
	if cluster == nil {
		log.Error(nil, "Missing cluster")

		return nil, errors.New("missing cluster")
	}
	if cluster.Spec.InfrastructureRef == nil {
		log.Error(nil, "Missing infrastructure reference")

		return nil, errors.New("missing infrastructure reference")
	}

	return cluster, nil
}

func (r *LinodeMachineReconciler) linodeClusterToLinodeMachines(logger logr.Logger) handler.MapFunc {
	logger = logger.WithName("LinodeMachineReconciler").WithName("linodeClusterToLinodeMachines")

	return func(ctx context.Context, o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		linodeCluster, ok := o.(*infrav1alpha2.LinodeCluster)
		if !ok {
			logger.Info("Failed to cast object to Cluster")

			return nil
		}

		if !linodeCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			logger.Info("Cluster has a deletion timestamp, skipping mapping")

			return nil
		}

		cluster, err := kutil.GetOwnerCluster(ctx, r.TracedClient(), linodeCluster.ObjectMeta)
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

func (r *LinodeMachineReconciler) requestsForCluster(ctx context.Context, namespace, name string) ([]ctrl.Request, error) {
	labels := map[string]string{clusterv1.ClusterNameLabel: name}

	machineList := clusterv1.MachineList{}
	if err := r.TracedClient().List(ctx, &machineList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
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

func (r *LinodeMachineReconciler) getPlacementGroupID(ctx context.Context, machineScope *scope.MachineScope, logger logr.Logger) (int, error) {
	name := machineScope.LinodeMachine.Spec.PlacementGroupRef.Name
	namespace := machineScope.LinodeMachine.Spec.PlacementGroupRef.Namespace
	if namespace == "" {
		namespace = machineScope.LinodeMachine.Namespace
	}

	logger = logger.WithValues("placementGroupName", name, "placementGroupNamespace", namespace)

	linodePlacementGroup := infrav1alpha2.LinodePlacementGroup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	objectKey := client.ObjectKeyFromObject(&linodePlacementGroup)
	err := r.Get(ctx, objectKey, &linodePlacementGroup)
	if err != nil {
		logger.Error(err, "Failed to fetch LinodePlacementGroup")
		return -1, err
	} else if !linodePlacementGroup.Status.Ready || linodePlacementGroup.Spec.PGID == nil {
		logger.Info("LinodePlacementGroup is not ready")
		return -1, errors.New("placement group is not ready")
	}

	return *linodePlacementGroup.Spec.PGID, nil
}

func (r *LinodeMachineReconciler) getVPCInterfaceConfig(ctx context.Context, machineScope *scope.MachineScope, logger logr.Logger) (*linodego.InstanceConfigInterfaceCreateOptions, error) {
	name := machineScope.LinodeCluster.Spec.VPCRef.Name
	namespace := machineScope.LinodeCluster.Spec.VPCRef.Namespace
	if namespace == "" {
		namespace = machineScope.LinodeCluster.Namespace
	}

	logger = logger.WithValues("vpcName", name, "vpcNamespace", namespace)

	linodeVPC := infrav1alpha2.LinodeVPC{
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

	var subnetID int
	vpc, err := machineScope.LinodeClient.GetVPC(ctx, *linodeVPC.Spec.VPCID)
	if err != nil {
		logger.Error(err, "Failed to fetch LinodeVPC")

		return nil, err
	}
	if vpc == nil {
		logger.Error(nil, "Failed to fetch VPC")

		return nil, errors.New("failed to fetch VPC")
	}
	if len(vpc.Subnets) == 0 {
		logger.Error(nil, "Failed to find subnet")

		return nil, errors.New("failed to find subnet")
	}
	// Place node into the least busy subnet
	sort.Slice(vpc.Subnets, func(i, j int) bool {
		return len(vpc.Subnets[i].Linodes) > len(vpc.Subnets[j].Linodes)
	})

	subnetID = vpc.Subnets[0].ID

	return &linodego.InstanceConfigInterfaceCreateOptions{
		Purpose:  linodego.InterfacePurposeVPC,
		Primary:  true,
		SubnetID: &subnetID,
		IPv4: &linodego.VPCIPv4{
			NAT1To1: ptr.To(("any")),
		},
	}, nil
}

func linodeMachineSpecToInstanceCreateConfig(machineSpec infrav1alpha2.LinodeMachineSpec) *linodego.InstanceCreateOptions {
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

func setUserData(ctx context.Context, machineScope *scope.MachineScope, createConfig *linodego.InstanceCreateOptions, logger logr.Logger) error {
	bootstrapData, err := machineScope.GetBootstrapData(ctx)
	if err != nil {
		logger.Error(err, "Failed to get bootstrap data")

		return err
	}
	if len(bootstrapData) > maxBootstrapDataBytes {
		err = errors.New("bootstrap data too large")
		logger.Error(err, "decoded bootstrap data exceeds size limit",
			"limit", maxBootstrapDataBytes,
		)

		return err
	}

	region, err := machineScope.LinodeClient.GetRegion(ctx, machineScope.LinodeMachine.Spec.Region)
	if err != nil {
		return fmt.Errorf("get region: %w", err)
	}
	regionMetadataSupport := slices.Contains(region.Capabilities, "Metadata")
	imageName := reconciler.DefaultMachineControllerLinodeImage
	if machineScope.LinodeMachine.Spec.Image != "" {
		imageName = machineScope.LinodeMachine.Spec.Image
	}
	image, err := machineScope.LinodeClient.GetImage(ctx, imageName)
	if err != nil {
		return fmt.Errorf("get image: %w", err)
	}
	imageMetadataSupport := slices.Contains(image.Capabilities, "cloud-init")
	if imageMetadataSupport && regionMetadataSupport {
		createConfig.Metadata = &linodego.InstanceMetadataOptions{
			UserData: b64.StdEncoding.EncodeToString(bootstrapData),
		}
	} else {
		logger.Info("using StackScripts for bootstrapping",
			"imageMetadataSupport", imageMetadataSupport,
			"regionMetadataSupport", regionMetadataSupport,
		)
		capiStackScriptID, err := services.EnsureStackscript(ctx, machineScope)
		if err != nil {
			return fmt.Errorf("ensure stackscript: %w", err)
		}
		createConfig.StackScriptID = capiStackScriptID
		// WARNING: label, region and type are currently supported as cloud-init variables,
		// any changes to this could be potentially backwards incompatible and should be noted through a backwards incompatible version update
		instanceData := fmt.Sprintf("label: %s\nregion: %s\ntype: %s", machineScope.LinodeMachine.Name, machineScope.LinodeMachine.Spec.Region, machineScope.LinodeMachine.Spec.Type)
		createConfig.StackScriptData = map[string]string{
			"instancedata": b64.StdEncoding.EncodeToString([]byte(instanceData)),
			"userdata":     b64.StdEncoding.EncodeToString(bootstrapData),
		}
	}
	return nil
}

func createInstanceConfigDeviceMap(instanceDisks map[string]*infrav1alpha2.InstanceDisk, instanceConfig *linodego.InstanceConfigDeviceMap) error {
	for deviceName, disk := range instanceDisks {
		dev := linodego.InstanceConfigDevice{
			DiskID: disk.DiskID,
		}
		switch deviceName {
		case "sdb":
			instanceConfig.SDB = &dev
		case "sdc":
			instanceConfig.SDC = &dev
		case "sdd":
			instanceConfig.SDD = &dev
		case "sde":
			instanceConfig.SDE = &dev
		case "sdf":
			instanceConfig.SDF = &dev
		case "sdg":
			instanceConfig.SDG = &dev
		case "sdh":
			instanceConfig.SDH = &dev
		default:
			return fmt.Errorf("unknown device name: %q", deviceName)
		}
	}

	return nil
}
