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
	"fmt"
	"net/netip"
	"slices"
	"sync"
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

func getClusterIPs(key string) *ClusterIPs {
	vlanIPsMu.Lock()
	defer vlanIPsMu.Unlock()
	ips, exists := vlanIPsMap[key]
	if !exists {
		ips = &ClusterIPs{
			ips: []string{},
		}
	}
	return ips
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
func GetNextVlanIP(clusterName, namespace string) string {
	key := fmt.Sprintf("%s.%s", namespace, clusterName)
	clusterIPs := getClusterIPs(key)
	return clusterIPs.getNextIP()
}

func DeleteClusterIPs(clusterName, namespace string) {
	vlanIPsMu.Lock()
	defer vlanIPsMu.Unlock()
	delete(vlanIPsMap, fmt.Sprintf("%s.%s", namespace, clusterName))
}
