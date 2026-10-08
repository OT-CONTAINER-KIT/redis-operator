package k8sutils

import (
	"fmt"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestGenerateContainerDefConfigVolumeMount(t *testing.T) {
	originalEnabled := features.Enabled(features.GenerateConfigInInitContainer)
	t.Cleanup(func() {
		require.NoError(t, features.MutableFeatureGate.Set(fmt.Sprintf("GenerateConfigInInitContainer=%t", originalEnabled)))
	})

	tests := []struct {
		name                     string
		role                     string
		generateConfigInInitCntr bool
		wantConfigMount          bool
	}{
		{
			name:                     "sentinel without init container keeps /etc/redis ephemeral so the image entrypoint can regenerate sentinel.conf on restart",
			role:                     "sentinel",
			generateConfigInInitCntr: false,
			wantConfigMount:          false,
		},
		{
			name:                     "redis without init container keeps /etc/redis ephemeral",
			role:                     "master",
			generateConfigInInitCntr: false,
			wantConfigMount:          false,
		},
		{
			name:                     "sentinel with init container persists /etc/redis",
			role:                     "sentinel",
			generateConfigInInitCntr: true,
			wantConfigMount:          true,
		},
		{
			name:                     "redis with init container persists /etc/redis",
			role:                     "master",
			generateConfigInInitCntr: true,
			wantConfigMount:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, features.MutableFeatureGate.Set(fmt.Sprintf("GenerateConfigInInitContainer=%t", tt.generateConfigInInitCntr)))

			containers := generateContainerDef(
				"redis",
				containerParameters{
					Role:            tt.role,
					Image:           "quay.io/opstree/redis:v8.2.2",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				false, false, false, nil, nil, nil, nil,
			)
			require.Len(t, containers, 1)

			var etcRedisMounts []corev1.VolumeMount
			for _, vm := range containers[0].VolumeMounts {
				if vm.MountPath == "/etc/redis" {
					etcRedisMounts = append(etcRedisMounts, vm)
				}
			}

			if !tt.wantConfigMount {
				assert.Empty(t, etcRedisMounts, "a persistent /etc/redis makes the image entrypoint append a second 'sentinel monitor' line on container restart (Duplicate master name)")
				return
			}
			require.Len(t, etcRedisMounts, 1)
			assert.Equal(t, common.VolumeNameConfig, etcRedisMounts[0].Name)
		})
	}
}
