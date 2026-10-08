package k8sutils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAlreadySlaveOf(t *testing.T) {
	mkInfo := func(lines ...string) string {
		return strings.Join(lines, "\r\n")
	}

	tests := []struct {
		name string
		info string
		host string
		port string
		want bool
	}{
		{
			name: "slave of correct master",
			info: mkInfo(
				"role:slave",
				"master_host:redis.example.com",
				"master_port:6379",
				"master_link_status:up",
			),
			host: "redis.example.com",
			port: "6379",
			want: true,
		},
		{
			name: "role is master",
			info: mkInfo(
				"role:master",
				"connected_slaves:0",
			),
			host: "redis.example.com",
			port: "6379",
			want: false,
		},
		{
			name: "slave of different host",
			info: mkInfo(
				"role:slave",
				"master_host:other.example.com",
				"master_port:6379",
			),
			host: "redis.example.com",
			port: "6379",
			want: false,
		},
		{
			name: "slave of different port",
			info: mkInfo(
				"role:slave",
				"master_host:redis.example.com",
				"master_port:7000",
			),
			host: "redis.example.com",
			port: "6379",
			want: false,
		},
		{
			name: "empty info",
			info: "",
			host: "redis.example.com",
			port: "6379",
			want: false,
		},
		{
			name: "slave with missing master_port line",
			info: mkInfo(
				"role:slave",
				"master_host:redis.example.com",
			),
			host: "redis.example.com",
			port: "6379",
			want: false,
		},
		{
			name: "matches with section header and trailing fields",
			info: mkInfo(
				"# Replication",
				"role:slave",
				"master_host:10.0.0.5",
				"master_port:6380",
				"slave_read_only:1",
			),
			host: "10.0.0.5",
			port: "6380",
			want: true,
		},
		{
			name: "cascade replica of pod-0",
			info: mkInfo(
				"role:slave",
				"master_host:redis-replication-0.redis-replication-headless.ns1.svc.cluster.local",
				"master_port:6379",
				"master_link_status:up",
			),
			host: "redis-replication-0.redis-replication-headless.ns1.svc.cluster.local",
			port: "6379",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAlreadySlaveOf(tt.info, tt.host, tt.port)
			assert.Equal(t, tt.want, got)
		})
	}
}
