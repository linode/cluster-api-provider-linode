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
	"math/rand/v2"
	"time"
)

const (
	// DefaultLoopTimeout is the default timeout for a reconcile loop.
	DefaultLoopTimeout = 90 * time.Minute
	// DefaultMappingTimeout is the default timeout for a controller request mapping func.
	DefaultMappingTimeout = 60 * time.Second

	// DefaultMachineControllerLinodeImage default image.
	DefaultMachineControllerLinodeImage = "linode/ubuntu22.04"
	// DefaultMachineControllerWaitForRunningDelay is the default requeue delay if instance is not running.
	DefaultMachineControllerWaitForRunningDelay = 25 * time.Second
	// DefaultMachineControllerWaitForPreflightTimeout is the default timeout during the preflight phase.
	DefaultMachineControllerWaitForPreflightTimeout = 5 * time.Minute
	// DefaultMachineControllerWaitForRunningTimeout is the default timeout if instance is not running.
	DefaultMachineControllerWaitForRunningTimeout = 20 * time.Minute
	// DefaultMachineControllerRetryDelay is the default requeue delay if there is an error.
	DefaultMachineControllerRetryDelay = 8 * time.Second
	// DefaultLinodeTooManyRequestsErrorRetryDelay is the default requeue delay if there is a Linode API error.
	DefaultLinodeTooManyRequestsErrorRetryDelay = time.Minute

	// DefaultVPCControllerReconcileDelay is the default requeue delay when a reconcile operation fails.
	DefaultVPCControllerReconcileDelay = 3 * time.Second
	// DefaultVPCControllerReconcileTimeout is the default timeout when VPC reconcile operations fail.
	DefaultVPCControllerReconcileTimeout = 20 * time.Minute
	// DefaultVPCControllerWaitForHasNodesTimeout is the default timeout if a VPC still has nodes.
	DefaultVPCControllerWaitForHasNodesTimeout = 20 * time.Minute

	// DefaultPGControllerReconcilerDelay is the default requeue delay when Placement Group reconcile operation fails.
	DefaultPGControllerReconcilerDelay = 3 * time.Second
	// DefaultPGControllerReconcileTimeout is the default timeout when Placement Group reconcile operations fail.
	DefaultPGControllerReconcileTimeout = 20 * time.Minute
	// DefaultPGControllerWaitForHasNodesTimeout is the default timeout when waiting for nodes attached to Placement Group.
	DefaultPGControllerWaitForHasNodesTimeout = 20 * time.Minute

	// DefaultFWControllerReconcilerDelay is the default requeue delay when a reconcile operation fails.
	DefaultFWControllerReconcilerDelay = 3 * time.Second
	// DefaultFWControllerReconcileTimeout is the default timeout when reconcile operations fail.
	DefaultFWControllerReconcileTimeout = 20 * time.Minute

	// DefaultClusterControllerReconcileDelay is the default requeue delay when a reconcile operation fails.
	DefaultClusterControllerReconcileDelay = 3 * time.Second
	// DefaultClusterControllerReconcileTimeout is the default timeout when reconcile operations fail.
	DefaultClusterControllerReconcileTimeout = 20 * time.Minute

	// DefaultObjectStorageBucketControllerReconcileDelay is the default requeue delay when a reconcile operation fails.
	DefaultObjectStorageBucketControllerReconcileDelay = 3 * time.Second

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

// RetryJitterFraction is the maximum fraction of positive jitter added on top of a retry delay.
// For example, a base delay of 8s with RetryJitterFraction=0.4 yields a range of [8s, 11.2s].
const RetryJitterFraction = 0.4

// WithJitter returns a duration with a random positive jitter applied to it, in the range [base, base*(1+RetryJitterFraction)].
func WithJitter(base time.Duration) time.Duration {
	maxJitter := time.Duration(float64(base) * RetryJitterFraction)
	// #nosec G404 - Jitter does not require cryptographic security
	return base + time.Duration(rand.Int64N(int64(maxJitter)+1))
}
