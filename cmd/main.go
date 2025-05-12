/*
Copyright 2020 Opstree Solutions.

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

package main

import (
	"os"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/cmd/agent"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/cmd/manager"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/monitoring"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var scheme = runtime.NewScheme()

//nolint:gochecknoinits
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(redisv1beta2.AddToScheme(scheme))
	monitoring.RegisterRedisReplicationMetrics()
	//+kubebuilder:scaffold:scheme
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "operator",
		Short: "Redis Operator for Kubernetes",
	}
	rootCmd.AddCommand(manager.CreateCommand(scheme))
	rootCmd.AddCommand(agent.CreateCommand())
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
