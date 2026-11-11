/*
Copyright 2023 Opstree Solutions.

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

// Package reconcileconfig holds the configurable reconcile requeue timing for
// the RedisReplication controller. The defaults reproduce the upstream
// hard-coded intervals exactly: 30s for the steady-state and statefulset-wait
// requeues and 60s for the failure/backoff requeues. Overrides are opt-in via
// manager flags, so an unconfigured operator behaves identically to upstream.
package reconcileconfig

import "time"

const (
	// DefaultRequeueInterval is the upstream steady-state requeue interval
	// (redisreplication_controller.go: the unconditional requeue and the
	// statefulset-not-ready waits).
	DefaultRequeueInterval = 30 * time.Second

	// DefaultFailureRequeueInterval is the upstream failure-path requeue
	// interval (redisreplication_controller.go: resource creation and
	// replication reconfiguration backoffs).
	DefaultFailureRequeueInterval = 60 * time.Second
)

// Config holds the RedisReplication reconcile requeue intervals.
type Config struct {
	// RequeueInterval is the steady-state requeue interval applied after a
	// successful reconcile and while waiting for a statefulset to become ready.
	RequeueInterval time.Duration

	// FailureRequeueInterval is the backoff interval applied after a resource
	// creation or replication reconfiguration error.
	FailureRequeueInterval time.Duration
}

// Default returns a Config whose intervals equal the upstream constants.
func Default() *Config {
	return &Config{
		RequeueInterval:        DefaultRequeueInterval,
		FailureRequeueInterval: DefaultFailureRequeueInterval,
	}
}

// Normalize returns a Config in which every non-positive interval is replaced
// by its upstream default, so a zero or negative flag value can never disable
// requeueing. A nil receiver yields the upstream default.
func (c *Config) Normalize() *Config {
	if c == nil {
		return Default()
	}

	normalized := &Config{
		RequeueInterval:        c.RequeueInterval,
		FailureRequeueInterval: c.FailureRequeueInterval,
	}
	if normalized.RequeueInterval <= 0 {
		normalized.RequeueInterval = DefaultRequeueInterval
	}
	if normalized.FailureRequeueInterval <= 0 {
		normalized.FailureRequeueInterval = DefaultFailureRequeueInterval
	}
	return normalized
}
