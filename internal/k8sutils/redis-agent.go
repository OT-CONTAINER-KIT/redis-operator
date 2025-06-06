package k8sutils

import (
	"fmt"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/image"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// RedisAgentConfig holds configuration for generating Redis agent sidecar
type RedisAgentConfig struct {
	Port                   *int
	ExistingPasswordSecret *common.ExistingPasswordSecret
}

// generateAgentSidecar creates a Redis agent sidecar configuration
// The agent currently performs role detection and may be extended with additional functionality in the future
func generateAgentSidecar(config RedisAgentConfig) common.Sidecar {
	operatorImage, _ := util.CoalesceEnv("OPERATOR_IMAGE", image.GetOperatorImage())
	redisPort := "6379"
	if config.Port != nil {
		redisPort = fmt.Sprintf("%d", *config.Port)
	}
	envVars := []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
	}

	// Add Redis password environment variable if configured
	if config.ExistingPasswordSecret != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "REDIS_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *config.ExistingPasswordSecret.Name,
					},
					Key: *config.ExistingPasswordSecret.Key,
				},
			},
		})
	}

	// Default resource requirements for role detector
	resources := &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("32Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
	}

	// Build complete command with arguments
	command := []string{
		"/operator",
		"agent",
		"server",
		"--redis-addr=127.0.0.1:" + redisPort,
		"--detect-interval=3s",
	}

	if config.ExistingPasswordSecret != nil {
		command = append(command, "--redis-password=$(REDIS_PASSWORD)")
	}

	return common.Sidecar{
		Name:            "agent",
		Image:           operatorImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         command,
		EnvVars:         &envVars,
		Resources:       resources,
		// SecurityContext can be omitted - the container image already runs as non-root
		// and inherits pod-level security context if needed
	}
}
