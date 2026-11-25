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

package reconcileconfig

import (
	"testing"
	"time"
)

// TestDefaultMatchesUpstreamConstants pins the default cadence to the upstream
// hard-coded values (30s steady-state, 60s failure path). The configurable
// requeue patch must not change default runtime behavior.
func TestDefaultMatchesUpstreamConstants(t *testing.T) {
	cfg := Default()

	if cfg.RequeueInterval != 30*time.Second {
		t.Errorf("Default().RequeueInterval = %v, want 30s (upstream constant)", cfg.RequeueInterval)
	}
	if cfg.FailureRequeueInterval != 60*time.Second {
		t.Errorf("Default().FailureRequeueInterval = %v, want 60s (upstream constant)", cfg.FailureRequeueInterval)
	}

	// The named constants must stay equal to the literal upstream cadence.
	if DefaultRequeueInterval != 30*time.Second {
		t.Errorf("DefaultRequeueInterval = %v, want 30s", DefaultRequeueInterval)
	}
	if DefaultFailureRequeueInterval != 60*time.Second {
		t.Errorf("DefaultFailureRequeueInterval = %v, want 60s", DefaultFailureRequeueInterval)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name        string
		input       *Config
		wantRequeue time.Duration
		wantFailure time.Duration
	}{
		{
			name:        "nil receiver yields upstream default",
			input:       nil,
			wantRequeue: DefaultRequeueInterval,
			wantFailure: DefaultFailureRequeueInterval,
		},
		{
			name: "positive overrides are preserved",
			input: &Config{
				RequeueInterval:        5 * time.Second,
				FailureRequeueInterval: 10 * time.Second,
			},
			wantRequeue: 5 * time.Second,
			wantFailure: 10 * time.Second,
		},
		{
			name: "zero intervals fall back to upstream default",
			input: &Config{
				RequeueInterval:        0,
				FailureRequeueInterval: 0,
			},
			wantRequeue: DefaultRequeueInterval,
			wantFailure: DefaultFailureRequeueInterval,
		},
		{
			name: "negative intervals fall back to upstream default",
			input: &Config{
				RequeueInterval:        -1 * time.Second,
				FailureRequeueInterval: -5 * time.Second,
			},
			wantRequeue: DefaultRequeueInterval,
			wantFailure: DefaultFailureRequeueInterval,
		},
		{
			name: "only the non-positive interval is replaced",
			input: &Config{
				RequeueInterval:        0,
				FailureRequeueInterval: 15 * time.Second,
			},
			wantRequeue: DefaultRequeueInterval,
			wantFailure: 15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.Normalize()
			if got.RequeueInterval != tt.wantRequeue {
				t.Errorf("RequeueInterval = %v, want %v", got.RequeueInterval, tt.wantRequeue)
			}
			if got.FailureRequeueInterval != tt.wantFailure {
				t.Errorf("FailureRequeueInterval = %v, want %v", got.FailureRequeueInterval, tt.wantFailure)
			}
		})
	}
}

func TestNormalizeDoesNotMutateReceiver(t *testing.T) {
	input := &Config{RequeueInterval: 0, FailureRequeueInterval: 0}
	_ = input.Normalize()

	if input.RequeueInterval != 0 || input.FailureRequeueInterval != 0 {
		t.Errorf("Normalize mutated the receiver: %+v", input)
	}
}
