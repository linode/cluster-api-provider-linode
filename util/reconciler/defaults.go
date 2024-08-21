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

package reconciler

import (
	"time"
)

const (
	// DefaultLoopTimeout is the default timeout for a reconcile loop.
	DefaultLoopTimeout = 90 * time.Minute
	// DefaultMappingTimeout is the default timeout for a controller request mapping func.
	DefaultMappingTimeout = 60 * time.Second

	// DefaultMachineControllerWaitForBootstrapDelay is the default requeue delay if bootstrap data is not ready.
	DefaultMachineControllerWaitForBootstrapDelay = 5 * time.Second
	// DefaultMachineControllerLinodeImage default image.
	DefaultMachineControllerLinodeImage = "linode/ubuntu22.04"
	// DefaultMachineControllerWaitForRunningDelay is the default requeue delay if instance is not running.
	DefaultMachineControllerWaitForRunningDelay = 15 * time.Second
	// DefaultMachineControllerWaitForPreflightTimeout is the default timeout during the preflight phase.
	DefaultMachineControllerWaitForPreflightTimeout = 5 * time.Minute
	// DefaultMachineControllerWaitForRunningTimeout is the default timeout if instance is not running.
	DefaultMachineControllerWaitForRunningTimeout = 20 * time.Minute
	// DefaultMachineControllerRetryDelay is the default requeue delay if there is an error.
	DefaultMachineControllerRetryDelay = 10 * time.Second
	// DefaultLinodeTooManyRequestsErrorRetryDelay is the default requeue delay if there is a Linode API error.
	DefaultLinodeTooManyRequestsErrorRetryDelay = time.Minute

	// DefaultVPCControllerReconcileDelay is the default requeue delay when a reconcile operation fails.
	DefaultVPCControllerReconcileDelay = 5 * time.Second
	// DefaultVPCControllerWaitForHasNodesTimeout is the default timeout when reconcile operations fail.
	DefaultVPCControllerReconcileTimeout = 20 * time.Minute
	// DefaultVPCControllerWaitForHasNodesDelay is the default requeue delay if a VPC has nodes.
	DefaultVPCControllerWaitForHasNodesDelay = 5 * time.Second
	// DefaultVPCControllerWaitForHasNodesTimeout is the default timeout if a VPC still has nodes.
	DefaultVPCControllerWaitForHasNodesTimeout = 20 * time.Minute

	// DefaultPGControllerReconcilerDelay is the default requeue delay when a reconcile operation fails.
	DefaultPGControllerReconcilerDelay = 5 * time.Second
	// DefaultPGControllerReconcileTimeout is the default timeout when reconcile operations fail.
	DefaultPGControllerReconcileTimeout = 20 * time.Minute

	// DefaultFWControllerReconcilerDelay is the default requeue delay when a reconcile operation fails.
	DefaultFWControllerReconcilerDelay = 5 * time.Second
	// DefaultFWControllerReconcileTimeout is the default timeout when reconcile operations fail.
	DefaultFWControllerReconcileTimeout = 20 * time.Minute

	// DefaultClusterControllerReconcileDelay is the default requeue delay when a reconcile operation fails.
	DefaultClusterControllerReconcileDelay = 5 * time.Second
	// DefaultClusterControllerReconcileTimeout is the default timeout when reconcile operations fail.
	DefaultClusterControllerReconcileTimeout = 20 * time.Minute

	// DefaultDNSTTLSec is the default TTL used for DNS entries for api server loadbalancing
	DefaultDNSTTLSec = 30
)

// DefaultedLoopTimeout will default the timeout if it is zero-valued.
func DefaultedLoopTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return DefaultLoopTimeout
	}

	return timeout
}

// DefaultTimeout returns timeout or backup if timeout is zero-valued.
func DefaultTimeout(timeout, backup time.Duration) time.Duration {
	if timeout <= 0 {
		return backup
	}

	return timeout
}
