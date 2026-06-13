package v1beta2

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestUseExternalMaster(t *testing.T) {
	tests := []struct {
		name string
		cr   *RedisReplication
		want bool
	}{
		{
			name: "nil receiver",
			cr:   nil,
			want: false,
		},
		{
			name: "no external master",
			cr:   &RedisReplication{},
			want: false,
		},
		{
			name: "external master set",
			cr: &RedisReplication{
				Spec: RedisReplicationSpec{
					ExternalMaster: &ExternalMaster{Host: "redis.example.com"},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cr.UseExternalMaster(); got != tt.want {
				t.Errorf("UseExternalMaster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetExternalMasterPort(t *testing.T) {
	tests := []struct {
		name string
		cr   *RedisReplication
		want int32
	}{
		{
			name: "default port when port nil",
			cr: &RedisReplication{
				Spec: RedisReplicationSpec{
					ExternalMaster: &ExternalMaster{Host: "redis.example.com"},
				},
			},
			want: 6379,
		},
		{
			name: "explicit port",
			cr: &RedisReplication{
				Spec: RedisReplicationSpec{
					ExternalMaster: &ExternalMaster{Host: "redis.example.com", Port: ptr.To(int32(7000))},
				},
			},
			want: 7000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cr.GetExternalMasterPort(); got != tt.want {
				t.Errorf("GetExternalMasterPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetExternalMasterEndpoint(t *testing.T) {
	tests := []struct {
		name string
		cr   *RedisReplication
		want string
	}{
		{
			name: "default port",
			cr: &RedisReplication{
				Spec: RedisReplicationSpec{
					ExternalMaster: &ExternalMaster{Host: "redis.example.com"},
				},
			},
			want: "redis.example.com:6379",
		},
		{
			name: "explicit port",
			cr: &RedisReplication{
				Spec: RedisReplicationSpec{
					ExternalMaster: &ExternalMaster{Host: "10.0.0.5", Port: ptr.To(int32(7001))},
				},
			},
			want: "10.0.0.5:7001",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cr.GetExternalMasterEndpoint(); got != tt.want {
				t.Errorf("GetExternalMasterEndpoint() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetConnectionInfoExternalMaster(t *testing.T) {
	// External master mode takes precedence and returns the external endpoint,
	// regardless of sentinel being set.
	cr := &RedisReplication{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
		Spec: RedisReplicationSpec{
			ExternalMaster: &ExternalMaster{Host: "redis.primary.example.com", Port: ptr.To(int32(6380))},
		},
	}
	info := cr.GetConnectionInfo("cluster.local")
	if info.Host != "redis.primary.example.com" {
		t.Errorf("GetConnectionInfo().Host = %q, want %q", info.Host, "redis.primary.example.com")
	}
	if info.Port != 6380 {
		t.Errorf("GetConnectionInfo().Port = %d, want %d", info.Port, 6380)
	}
	if info.MasterName != "" {
		t.Errorf("GetConnectionInfo().MasterName = %q, want empty", info.MasterName)
	}
}
