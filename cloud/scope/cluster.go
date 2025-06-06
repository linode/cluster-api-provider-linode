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
	"github.com/linode/cluster-api-provider-linode/clients"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	Client            clients.K8sClient
	Cluster           *clusterv1.Cluster
	LinodeCluster     *infrav1alpha2.LinodeCluster
	LinodeMachineList infrav1alpha2.LinodeMachineList
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
func NewClusterScope(ctx context.Context, linodeClientConfig, dnsClientConfig ClientConfig, params ClusterScopeParams) (*ClusterScope, error) {
	if err := validateClusterScopeParams(params); err != nil {
		return nil, err
	}

	linodeClient, err := CreateLinodeClient(linodeClientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}

	helper, err := patch.NewHelper(params.LinodeCluster, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	akamDomainsClient, err := setUpEdgeDNSInterface()
	if err != nil {
		return nil, fmt.Errorf("failed to create akamai dns client: %w", err)
	}
	linodeDomainsClient, err := CreateLinodeClient(dnsClientConfig, WithRetryCount(0))
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client: %w", err)
	}

	return &ClusterScope{
		Client:              params.Client,
		Cluster:             params.Cluster,
		LinodeClient:        linodeClient,
		LinodeDomainsClient: linodeDomainsClient,
		AkamaiDomainsClient: akamDomainsClient,
		LinodeCluster:       params.LinodeCluster,
		LinodeMachines:      params.LinodeMachineList,
		PatchHelper:         helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	Client              clients.K8sClient
	PatchHelper         *patch.Helper
	LinodeClient        clients.LinodeClient
	Cluster             *clusterv1.Cluster
	LinodeCluster       *infrav1alpha2.LinodeCluster
	LinodeMachines      infrav1alpha2.LinodeMachineList
	AkamaiDomainsClient clients.AkamClient
	LinodeDomainsClient clients.LinodeClient
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

func (s *ClusterScope) SetCredentialRefTokenForLinodeClients(ctx context.Context) error {
	if s.LinodeCluster.Spec.CredentialsRef != nil {
		apiToken, err := getCredentialDataFromRef(ctx, s.Client, *s.LinodeCluster.Spec.CredentialsRef, s.LinodeCluster.GetNamespace(), "apiToken")
		if err != nil {
			return fmt.Errorf("credentials from secret ref: %w", err)
		}
		s.LinodeClient = s.LinodeClient.SetToken(string(apiToken))
		dnsToken, err := getCredentialDataFromRef(ctx, s.Client, *s.LinodeCluster.Spec.CredentialsRef, s.LinodeCluster.GetNamespace(), "dnsToken")
		if err != nil || len(dnsToken) == 0 {
			dnsToken = apiToken
		}
		s.LinodeDomainsClient = s.LinodeDomainsClient.SetToken(string(dnsToken))
		return nil
	}
	return nil
}
