package v1beta2_test

import (
	"testing"

	common "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	v1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestRedisReplicationSpec_GetRedisDynamicConfig(t *testing.T) {
	tests := []struct {
		name string
		spec v1beta2.RedisReplicationSpec
		want []string
	}{
		{
			name: "nil RedisConfig returns empty slice",
			spec: v1beta2.RedisReplicationSpec{},
			want: []string{},
		},
		{
			name: "RedisConfig without dynamic config returns empty slice",
			spec: v1beta2.RedisReplicationSpec{
				RedisConfig: &common.RedisConfig{},
			},
			want: []string{},
		},
		{
			name: "empty dynamic config returns empty slice",
			spec: v1beta2.RedisReplicationSpec{
				RedisConfig: &common.RedisConfig{DynamicConfig: []string{}},
			},
			want: []string{},
		},
		{
			name: "dynamic config is returned",
			spec: v1beta2.RedisReplicationSpec{
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

func TestUseExternalMaster(t *testing.T) {
	tests := []struct {
		name string
		cr   *v1beta2.RedisReplication
		want bool
	}{
		{
			name: "nil receiver",
			cr:   nil,
			want: false,
		},
		{
			name: "no external master",
			cr:   &v1beta2.RedisReplication{},
			want: false,
		},
		{
			name: "external master set",
			cr: &v1beta2.RedisReplication{
				Spec: v1beta2.RedisReplicationSpec{
					ExternalMaster: &v1beta2.ExternalMaster{Host: "redis.example.com"},
				},
			},
			want: true,
		},
		{
			name: "external master with empty host",
			cr: &v1beta2.RedisReplication{
				Spec: v1beta2.RedisReplicationSpec{
					ExternalMaster: &v1beta2.ExternalMaster{Host: ""},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cr.UseExternalMaster())
		})
	}
}

func TestGetExternalMasterPort(t *testing.T) {
	tests := []struct {
		name string
		cr   *v1beta2.RedisReplication
		want int32
	}{
		{
			name: "port is nil defaults to 6379",
			cr: &v1beta2.RedisReplication{
				Spec: v1beta2.RedisReplicationSpec{
					ExternalMaster: &v1beta2.ExternalMaster{Host: "redis.example.com"},
				},
			},
			want: 6379,
		},
		{
			name: "explicit port",
			cr: &v1beta2.RedisReplication{
				Spec: v1beta2.RedisReplicationSpec{
					ExternalMaster: &v1beta2.ExternalMaster{
						Host: "redis.example.com",
						Port: ptr.To(int32(7000)),
					},
				},
			},
			want: 7000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cr.GetExternalMasterPort())
		})
	}
}

func TestGetExternalMasterEndpoint(t *testing.T) {
	tests := []struct {
		name string
		cr   *v1beta2.RedisReplication
		want string
	}{
		{
			name: "default port",
			cr: &v1beta2.RedisReplication{
				Spec: v1beta2.RedisReplicationSpec{
					ExternalMaster: &v1beta2.ExternalMaster{Host: "redis.example.com"},
				},
			},
			want: "redis.example.com:6379",
		},
		{
			name: "explicit port",
			cr: &v1beta2.RedisReplication{
				Spec: v1beta2.RedisReplicationSpec{
					ExternalMaster: &v1beta2.ExternalMaster{
						Host: "10.0.0.5",
						Port: ptr.To(int32(7001)),
					},
				},
			},
			want: "10.0.0.5:7001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.cr.GetExternalMasterEndpoint())
		})
	}
}

func TestGetConnectionInfoExternalMaster(t *testing.T) {
	cr := &v1beta2.RedisReplication{
		Spec: v1beta2.RedisReplicationSpec{
			ExternalMaster: &v1beta2.ExternalMaster{
				Host: "redis.primary.example.com",
				Port: ptr.To(int32(6380)),
			},
		},
	}

	info := cr.GetConnectionInfo("cluster.local")

	assert.Equal(t, "redis.primary.example.com", info.Host)
	assert.Equal(t, 6380, info.Port)
	assert.Empty(t, info.MasterName)
}
