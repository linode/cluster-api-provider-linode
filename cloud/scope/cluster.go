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

package scope

import (
	"context"
	"errors"
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1alpha2 "github.com/linode/cluster-api-provider-linode/api/v1alpha2"

	. "github.com/linode/cluster-api-provider-linode/clients"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	Client        K8sClient
	Cluster       *clusterv1.Cluster
	LinodeCluster *infrav1alpha2.LinodeCluster
}

func validateClusterScopeParams(params ClusterScopeParams) error {
	if params.Cluster == nil {
		return errors.New("cluster is required when creating a ClusterScope")
	}
	if params.LinodeCluster == nil {
		return errors.New("linodeCluster is required when creating a ClusterScope")
	}

	return nil
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(ctx context.Context, apiKey string, params ClusterScopeParams) (*ClusterScope, error) {
	if err := validateClusterScopeParams(params); err != nil {
		return nil, err
	}

	// Override the controller credentials with ones from the Cluster's Secret reference (if supplied).
	if params.LinodeCluster.Spec.CredentialsRef != nil {
		// TODO: This key is hard-coded (for now) to match the externally-managed `manager-credentials` Secret.
		apiToken, err := getCredentialDataFromRef(ctx, params.Client, *params.LinodeCluster.Spec.CredentialsRef, params.LinodeCluster.GetNamespace(), "apiToken")
		if err != nil {
			return nil, fmt.Errorf("error get data for key apiToken")
		}
		apiKey = string(apiToken)
	}
	linodeClient, err := CreateLinodeClient(apiKey, defaultClientTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}

	helper, err := patch.NewHelper(params.LinodeCluster, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &ClusterScope{
		Client:        params.Client,
		Cluster:       params.Cluster,
		LinodeClient:  linodeClient,
		LinodeCluster: params.LinodeCluster,
		PatchHelper:   helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	Client        K8sClient
	PatchHelper   *patch.Helper
	LinodeClient  LinodeClient
	Cluster       *clusterv1.Cluster
	LinodeCluster *infrav1alpha2.LinodeCluster
}

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	return s.PatchHelper.Patch(ctx, s.LinodeCluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AddFinalizer adds a finalizer if not present and immediately patches the
// object to avoid any race conditions.
func (s *ClusterScope) AddFinalizer(ctx context.Context) error {
	if controllerutil.AddFinalizer(s.LinodeCluster, infrav1alpha2.ClusterFinalizer) {
		return s.Close(ctx)
	}

	return nil
}

func (s *ClusterScope) AddCredentialsRefFinalizer(ctx context.Context) error {
	if s.LinodeCluster.Spec.CredentialsRef == nil {
		return nil
	}

	return addCredentialsFinalizer(ctx, s.Client,
		*s.LinodeCluster.Spec.CredentialsRef, s.LinodeCluster.GetNamespace(),
		toFinalizer(s.LinodeCluster))
}

func (s *ClusterScope) RemoveCredentialsRefFinalizer(ctx context.Context) error {
	if s.LinodeCluster.Spec.CredentialsRef == nil {
		return nil
	}

	return removeCredentialsFinalizer(ctx, s.Client,
		*s.LinodeCluster.Spec.CredentialsRef, s.LinodeCluster.GetNamespace(),
		toFinalizer(s.LinodeCluster))
}
