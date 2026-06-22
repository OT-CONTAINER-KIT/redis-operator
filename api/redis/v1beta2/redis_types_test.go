package v1beta2_test

import (
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	v1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestRedisSpec_GetRedisDynamicConfig(t *testing.T) {
	tests := []struct {
		name string
		spec v1beta2.RedisSpec
		want []string
	}{
		{
			name: "nil RedisConfig returns empty slice",
			spec: v1beta2.RedisSpec{},
			want: []string{},
		},
		{
			name: "RedisConfig without dynamic config returns empty slice",
			spec: v1beta2.RedisSpec{
				RedisConfig: &common.RedisConfig{},
			},
			want: []string{},
		},
		{
			name: "empty dynamic config returns empty slice",
			spec: v1beta2.RedisSpec{
				RedisConfig: &common.RedisConfig{DynamicConfig: []string{}},
			},
			want: []string{},
		},
		{
			name: "dynamic config is returned",
			spec: v1beta2.RedisSpec{
				RedisConfig: &common.RedisConfig{
					DynamicConfig: []string{"maxmemory-policy allkeys-lru", "slowlog-log-slower-than 5000"},
				},
			},
			want: []string{"maxmemory-policy allkeys-lru", "slowlog-log-slower-than 5000"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.spec.GetRedisDynamicConfig())
		})
	}
}
