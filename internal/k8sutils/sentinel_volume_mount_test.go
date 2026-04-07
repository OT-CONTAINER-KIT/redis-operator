package k8sutils

import (
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/features"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestSentinelContainerMountsConfigVolume(t *testing.T) {
	// Ensure GenerateConfigInInitContainer is disabled (default)
	if err := features.MutableFeatureGate.Set("GenerateConfigInInitContainer=false"); err != nil {
		t.Fatalf("failed to set feature gate: %v", err)
	}

	containers := generateContainerDef(
		"redis-sentinel-sentinel",
		containerParameters{
			Role:            "sentinel",
			Image:           "quay.io/opstree/redis-sentinel:v8.2.2",
			ImagePullPolicy: corev1.PullIfNotPresent,
		},
		false, // clusterMode
		false, // nodeConfVolume
		false, // enableMetrics
		nil,   // externalConfig
		nil,   // clusterVersion
		nil,   // mountpath
		nil,   // sidecars
	)

	assert.Len(t, containers, 1, "should have exactly one container")

	// Verify the config volume is mounted at /etc/redis
	var found bool
	for _, vm := range containers[0].VolumeMounts {
		if vm.Name == common.VolumeNameConfig && vm.MountPath == "/etc/redis" {
			found = true
			break
		}
	}
	assert.True(t, found, "sentinel container should mount the config volume at /etc/redis for sentinel.conf persistence")
}

func TestNonSentinelContainerDoesNotMountConfigVolumeByDefault(t *testing.T) {
	// Ensure GenerateConfigInInitContainer is disabled (default)
	if err := features.MutableFeatureGate.Set("GenerateConfigInInitContainer=false"); err != nil {
		t.Fatalf("failed to set feature gate: %v", err)
	}

	containers := generateContainerDef(
		"redis",
		containerParameters{
			Role:            "master",
			Image:           "redis:latest",
			ImagePullPolicy: corev1.PullAlways,
		},
		false, false, false, nil, nil, nil, nil,
	)

	assert.Len(t, containers, 1)

	for _, vm := range containers[0].VolumeMounts {
		if vm.Name == common.VolumeNameConfig && vm.MountPath == "/etc/redis" {
			t.Error("non-sentinel container should NOT mount config volume when GenerateConfigInInitContainer is disabled")
		}
	}
}
