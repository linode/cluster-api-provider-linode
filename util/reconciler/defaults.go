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
	// DefaultMachineControllerWaitForRunningDelay is the default requeue delay if instance is not running.
	DefaultMachineControllerWaitForRunningDelay = 5 * time.Second
	// DefaultMachineControllerWaitForRunningTimeout is the default timeout if instance is not running.
	DefaultMachineControllerWaitForRunningTimeout = 20 * time.Minute

	// DefaultVPCControllerWaitForHasNodesDelay is the default requeue delay if VPC has nodes.
	DefaultVPCControllerWaitForHasNodesDelay = 5 * time.Second
	// DefaultVPCControllerWaitForHasNodesTimeout is the default timeout if instance is not running.
	DefaultVPCControllerWaitForHasNodesTimeout = 20 * time.Minute
)

// DefaultedLoopTimeout will default the timeout if it is zero-valued.
func DefaultedLoopTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return DefaultLoopTimeout
	}

	return timeout
}
