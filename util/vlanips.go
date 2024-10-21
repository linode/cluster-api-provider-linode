/*
Copyright 2024 Akamai Technologies, Inc.

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

package util

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/linode/cluster-api-provider-linode/api/v1alpha2"
)

var (
	vlanIPsMu sync.RWMutex
	// vlanIPsMap stores clusterName and a list of VlanIPs assigned to that cluster
	vlanIPsMap  = make(map[string]*ClusterIPs, 0)
	vlanIPRange = "10.0.0.0/8"
)

type ClusterIPs struct {
	mu  sync.RWMutex
	ips []string
}

func getExistingIPsForCluster(ctx context.Context, clusterName, namespace string, kubeclient client.Client) ([]string, error) {
	clusterReq, err := labels.NewRequirement("cluster.x-k8s.io/cluster-name", selection.Equals, []string{clusterName})
	if err != nil {
		return nil, fmt.Errorf("building label selector: %w", err)
	}

	selector := labels.NewSelector()
	selector = selector.Add(*clusterReq)
	var linodeMachineList v1alpha2.LinodeMachineList
	err = kubeclient.List(ctx, &linodeMachineList, &client.ListOptions{Namespace: namespace, LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("listing all linodeMachines %w", err)
	}

	_, ipnet, err := net.ParseCIDR(vlanIPRange)
	if err != nil {
		return nil, fmt.Errorf("parsing vlanIPRange: %w", err)
	}

	existingIPs := []string{}
	for _, lm := range linodeMachineList.Items {
		for _, addr := range lm.Status.Addresses {
			if addr.Type == clusterv1.MachineInternalIP && ipnet.Contains(net.ParseIP(addr.Address)) {
				existingIPs = append(existingIPs, addr.Address)
			}
		}
	}
	return existingIPs, nil
}

func getClusterIPs(ctx context.Context, clusterName, namespace string, kubeclient client.Client) (*ClusterIPs, error) {
	key := fmt.Sprintf("%s.%s", namespace, clusterName)
	vlanIPsMu.Lock()
	defer vlanIPsMu.Unlock()
	clusterIps, exists := vlanIPsMap[key]
	if !exists {
		ips, err := getExistingIPsForCluster(ctx, clusterName, namespace, kubeclient)
		if err != nil {
			return nil, fmt.Errorf("getting existingIPs for a cluster: %w", err)
		}
		clusterIps = &ClusterIPs{
			ips: ips,
		}
		vlanIPsMap[key] = clusterIps
	}
	return clusterIps, nil
}

func (c *ClusterIPs) getNextIP() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	prefix := netip.MustParsePrefix(vlanIPRange)
	currentIp := prefix.Addr().Next()

	ipString := currentIp.String()
	for {
		if !slices.Contains(c.ips, ipString) {
			break
		}
		currentIp = currentIp.Next()
		ipString = currentIp.String()
	}
	c.ips = append(c.ips, ipString)
	return ipString
}

// GetNextVlanIP returns the next available IP for a cluster
func GetNextVlanIP(ctx context.Context, clusterName, namespace string, kubeclient client.Client) (string, error) {
	clusterIPs, err := getClusterIPs(ctx, clusterName, namespace, kubeclient)
	if err != nil {
		return "", err
	}
	return clusterIPs.getNextIP(), nil
}

func DeleteClusterIPs(clusterName, namespace string) {
	vlanIPsMu.Lock()
	defer vlanIPsMu.Unlock()
	delete(vlanIPsMap, fmt.Sprintf("%s.%s", namespace, clusterName))
}
