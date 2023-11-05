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
	"fmt"
	"net/http"
	"os"

	infrav1 "github.com/linode/cluster-api-provider-linode/api/v1alpha1"
	"github.com/linode/linodego"
	"golang.org/x/oauth2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	Client        client.Client
	Cluster       *clusterv1.Cluster
	LinodeClient  *linodego.Client
	LinodeCluster *infrav1.LinodeCluster
}

func validateClusterScopeParams(params ClusterScopeParams) error {
	if params.Cluster == nil {
		return fmt.Errorf("Cluster is required when creating a ClusterScope")
	}
	if params.LinodeCluster == nil {
		return fmt.Errorf("LinodeCluster is required when creating a ClusterScope")
	}
	return nil
}

func createLinodeClient() (*linodego.Client, error) {
	apiKey, ok := os.LookupEnv("LINODE_TOKEN")
	if !ok {
		return nil, fmt.Errorf("failed to get LINODE_TOKEN environment variable")
	}
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}
	linodeClient := linodego.NewClient(oauth2Client)
	return &linodeClient, nil
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(ctx context.Context, params ClusterScopeParams) (*ClusterScope, error) {
	// TODO
	if err := validateClusterScopeParams(params); err != nil {
		return nil, err
	}

	if params.LinodeClient == nil {
		if linodeClient, err := createLinodeClient(); err != nil {
			return nil, err
		} else {
			params.LinodeClient = linodeClient
		}
	}

	helper, err := patch.NewHelper(params.LinodeCluster, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &ClusterScope{
		client:        params.Client,
		Cluster:       params.Cluster,
		LinodeClient:  params.LinodeClient,
		LinodeCluster: params.LinodeCluster,
		patchHelper:   helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	client      client.Client
	patchHelper *patch.Helper

	LinodeClient  *linodego.Client
	Cluster       *clusterv1.Cluster
	LinodeCluster *infrav1.LinodeCluster
}
