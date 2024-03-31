package k8sutils

import (
	"context"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/api"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	mock_utils "github.com/OT-CONTAINER-KIT/redis-operator/mocks/utils"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/go-redis/redismock/v9"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func TestCheckRedisNodePresence(t *testing.T) {
	cr := &redisv1beta2.RedisCluster{}
	output := "205dd1780dda981f9320c9d47d069b3c0ceaa358 172.17.0.24:6379@16379 slave b65312dcf5537b8826c344783f078096fdb7f27c 0 1654197347000 1 connected\nfaa21623054227826e93dd71314cce3706491dac 172.17.0.28:6379@16379 slave d54557b21bc5a5aa947ce58b7dbadc5d39bdd551 0 1654197347000 2 connected\nb65312dcf5537b8826c344783f078096fdb7f27c 172.17.0.25:6379@16379 master - 0 1654197346000 1 connected 0-5460\nd54557b21bc5a5aa947ce58b7dbadc5d39bdd551 172.17.0.29:6379@16379 myself,master - 0 1654197347000 2 connected 5461-10922\nc9fa05269c4e662295bf34eb93f1315f962493ba 172.17.0.3:6379@16379 master - 0 1654197348006 3 connected 10923-16383"
	csvOutput := csv.NewReader(strings.NewReader(output))
	csvOutput.Comma = ' '
	csvOutput.FieldsPerRecord = -1
	nodes, _ := csvOutput.ReadAll()

	tests := []struct {
		nodes [][]string
		ip    string
		want  bool
	}{
		{nodes, "172.17.0.24", true},
		{nodes, "172.17.0.111", false},
		{nodes, "172.17.0.2", false},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%s,%s", tt.nodes, tt.ip)
		t.Run(testname, func(t *testing.T) {
			ans := checkRedisNodePresence(cr, tt.nodes, tt.ip)
			if ans != tt.want {
				t.Errorf("got %t, want %t", ans, tt.want)
			}
		})
	}
}

func TestGetRedisServerIP(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *k8sClientFake.Clientset
		redisInfo   RedisDetails
		expectedIP  string
		expectEmpty bool
	}{
		{
			name: "Successfully retrieve IPv4 address",
			setup: func() *k8sClientFake.Clientset {
				return k8sClientFake.NewSimpleClientset(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						PodIP: "192.168.1.1",
					},
				})
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectedIP:  "192.168.1.1",
			expectEmpty: false,
		},
		{
			name: "Successfully retrieve IPv6 address",
			setup: func() *k8sClientFake.Clientset {
				return k8sClientFake.NewSimpleClientset(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						PodIP: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
					},
				})
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectedIP:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expectEmpty: false,
		},
		{
			name: "Error retrieving pod results in empty IP",
			setup: func() *k8sClientFake.Clientset {
				client := k8sClientFake.NewSimpleClientset()
				return client
			},
			redisInfo: RedisDetails{
				PodName:   "nonexistent-pod",
				Namespace: "default",
			},
			expectEmpty: true,
		},
		{
			name: "Empty results in empty IP",
			setup: func() *k8sClientFake.Clientset {
				return k8sClientFake.NewSimpleClientset(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						PodIP: "",
					},
				})
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup()
			logger := testr.New(t)
			redisIP := getRedisServerIP(client, logger, tt.redisInfo)

			if tt.expectEmpty {
				assert.Empty(t, redisIP, "Expected an empty IP address")
			} else {
				assert.Equal(t, tt.expectedIP, redisIP, "Expected and actual IP do not match")
			}
		})
	}
}

func TestGetRedisServerAddress(t *testing.T) {
	tests := []struct {
		name         string
		setup        func() *k8sClientFake.Clientset
		redisInfo    RedisDetails
		expectedAddr string
		expectEmpty  bool
	}{
		{
			name: "Successfully retrieve IPv4 URI",
			setup: func() *k8sClientFake.Clientset {
				return k8sClientFake.NewSimpleClientset(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						PodIP: "192.168.1.1",
					},
				})
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectedAddr: "192.168.1.1:6379",
			expectEmpty:  false,
		},
		{
			name: "Successfully retrieve IPv6 URI",
			setup: func() *k8sClientFake.Clientset {
				return k8sClientFake.NewSimpleClientset(&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "redis-pod",
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						PodIP: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
					},
				})
			},
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			expectedAddr: "[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:6379",
			expectEmpty:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setup()
			logger := testr.New(t)
			redisIP := getRedisServerAddress(client, logger, tt.redisInfo, 6379)

			if tt.expectEmpty {
				assert.Empty(t, redisIP, "Expected an empty address")
			} else {
				assert.Equal(t, tt.expectedAddr, redisIP, "Expected and actual address do not match")
			}
		})
	}
}

func TestGetRedisHostname(t *testing.T) {
	tests := []struct {
		name         string
		redisInfo    RedisDetails
		redisCluster *redisv1beta2.RedisCluster
		role         string
		expected     string
	}{
		{
			name: "standard configuration",
			redisInfo: RedisDetails{
				PodName:   "redis-pod",
				Namespace: "default",
			},
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mycluster",
					Namespace: "default",
				},
			},
			role:     "master",
			expected: "redis-pod.mycluster-master-headless.default.svc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fqdn := getRedisHostname(tt.redisInfo, tt.redisCluster, tt.role)
			assert.Equal(t, tt.expected, fqdn, "FQDN should match the expected output")
		})
	}
}

func TestCreateSingleLeaderRedisCommand(t *testing.T) {
	logger := testr.New(t)
	cr := &redisv1beta2.RedisCluster{}
	cmd := CreateSingleLeaderRedisCommand(logger, cr)

	assert.Equal(t, "redis-cli", cmd[0])
	assert.Equal(t, "CLUSTER", cmd[1])
	assert.Equal(t, "ADDSLOTS", cmd[2])

	expectedLength := 16384 + 3

	assert.Equal(t, expectedLength, len(cmd))
	assert.Equal(t, "0", cmd[3])
	assert.Equal(t, "16383", cmd[expectedLength-1])
}

func TestCreateMultipleLeaderRedisCommand(t *testing.T) {
	tests := []struct {
		name             string
		redisCluster     *redisv1beta2.RedisCluster
		expectedCommands []string
	}{
		{
			name: "Multiple leaders cluster version v7",
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mycluster",
					Namespace: "default",
				},
				Spec: redisv1beta2.RedisClusterSpec{
					Size:           ptr.To(int32(3)),
					ClusterVersion: ptr.To("v7"),
					Port:           ptr.To(6379),
				},
			},
			expectedCommands: []string{
				"redis-cli", "--cluster", "create",
				"mycluster-leader-0.mycluster-leader-headless.default.svc:6379",
				"mycluster-leader-1.mycluster-leader-headless.default.svc:6379",
				"mycluster-leader-2.mycluster-leader-headless.default.svc:6379",
				"--cluster-yes",
			},
		},
		{
			name: "Multiple leaders cluster without version v7",
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mycluster",
					Namespace: "default",
				},
				Spec: redisv1beta2.RedisClusterSpec{
					Size: ptr.To(int32(3)),
					Port: ptr.To(6379),
				},
			},
			expectedCommands: []string{
				"redis-cli", "--cluster", "create",
				"192.168.1.1:6379",
				"192.168.1.2:6379",
				"192.168.1.3:6379",
				"--cluster-yes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mock_utils.CreateFakeClientWithPodIPs_LeaderPods(tt.redisCluster)
			logger := testr.New(t)

			cmd := CreateMultipleLeaderRedisCommand(client, logger, tt.redisCluster)
			assert.Equal(t, tt.expectedCommands, cmd)
		})
	}
}

func TestGetRedisTLSArgs(t *testing.T) {
	tests := []struct {
		name       string
		tlsConfig  *redisv1beta2.TLSConfig
		clientHost string
		expected   []string
	}{
		{
			name:       "with TLS configuration",
			tlsConfig:  &redisv1beta2.TLSConfig{},
			clientHost: "redis-host",
			expected:   []string{"--tls", "--cacert", "/tls/ca.crt", "-h", "redis-host"},
		},
		{
			name:       "without TLS configuration",
			tlsConfig:  nil,
			clientHost: "redis-host",
			expected:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := getRedisTLSArgs(tt.tlsConfig, tt.clientHost)
			assert.Equal(t, tt.expected, cmd, "Expected command arguments do not match")
		})
	}
}

func TestCreateRedisReplicationCommand(t *testing.T) {
	logger := logr.Discard()
	type secret struct {
		name      string
		namespace string
		key       string
	}
	tests := []struct {
		name         string
		redisCluster *redisv1beta2.RedisCluster
		secret
		leaderPod       RedisDetails
		followerPod     RedisDetails
		expectedCommand []string
	}{
		{
			name: "Test case with cluster version v7",
			secret: secret{
				name:      "redis-password-secret",
				namespace: "default",
				key:       "password",
			},
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: redisv1beta2.RedisClusterSpec{
					Size: ptr.To(int32(3)),
					KubernetesConfig: redisv1beta2.KubernetesConfig{
						KubernetesConfig: api.KubernetesConfig{
							ExistingPasswordSecret: &api.ExistingPasswordSecret{
								Name: ptr.To("redis-password-secret"),
								Key:  ptr.To("password"),
							},
						},
					},
					ClusterVersion: ptr.To("v7"),
					Port:           ptr.To(6379),
				},
			},
			leaderPod: RedisDetails{
				PodName:   "redis-cluster-leader-0",
				Namespace: "default",
			},
			followerPod: RedisDetails{
				PodName:   "redis-cluster-follower-0",
				Namespace: "default",
			},
			expectedCommand: []string{
				"redis-cli", "--cluster", "add-node",
				"redis-cluster-follower-0.redis-cluster-follower-headless.default.svc:6379",
				"redis-cluster-leader-0.redis-cluster-leader-headless.default.svc:6379",
				"--cluster-slave",
				"-a", "password",
			},
		},
		{
			name: "Test case with cluster version v7 failed to get password",
			secret: secret{
				name:      "wrong-name",
				namespace: "default",
				key:       "password",
			},
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: redisv1beta2.RedisClusterSpec{
					Size: ptr.To(int32(3)),
					KubernetesConfig: redisv1beta2.KubernetesConfig{
						KubernetesConfig: api.KubernetesConfig{
							ExistingPasswordSecret: &api.ExistingPasswordSecret{
								Name: ptr.To("redis-password-secret"),
								Key:  ptr.To("password"),
							},
						},
					},
					ClusterVersion: ptr.To("v7"),
					Port:           ptr.To(6379),
				},
			},
			leaderPod: RedisDetails{
				PodName:   "redis-cluster-leader-0",
				Namespace: "default",
			},
			followerPod: RedisDetails{
				PodName:   "redis-cluster-follower-0",
				Namespace: "default",
			},
			expectedCommand: []string{
				"redis-cli", "--cluster", "add-node",
				"redis-cluster-follower-0.redis-cluster-follower-headless.default.svc:6379",
				"redis-cluster-leader-0.redis-cluster-leader-headless.default.svc:6379",
				"--cluster-slave",
			},
		},
		{
			name: "Test case without cluster version v7",
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
				Spec: redisv1beta2.RedisClusterSpec{
					Size: ptr.To(int32(3)),
					Port: ptr.To(6379),
				},
			},
			leaderPod: RedisDetails{
				PodName:   "redis-cluster-leader-0",
				Namespace: "default",
			},
			followerPod: RedisDetails{
				PodName:   "redis-cluster-follower-0",
				Namespace: "default",
			},
			expectedCommand: []string{
				"redis-cli", "--cluster", "add-node",
				"192.168.2.1:6379",
				"192.168.1.1:6379",
				"--cluster-slave",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pods := mock_utils.CreateFakeObjectWithPodIPs(tt.redisCluster)
			var secret []runtime.Object
			if tt.redisCluster.Spec.KubernetesConfig.ExistingPasswordSecret != nil {
				secret = mock_utils.CreateFakeObjectWithSecret(tt.secret.name, tt.secret.namespace, tt.secret.key)
			}

			var objects []runtime.Object
			objects = append(objects, pods...)
			objects = append(objects, secret...)

			client := fake.NewSimpleClientset(objects...)
			cmd := createRedisReplicationCommand(client, logger, tt.redisCluster, tt.leaderPod, tt.followerPod)

			// Assert the command is as expected using testify
			assert.Equal(t, tt.expectedCommand, cmd)
		})
	}
}

func TestGetContainerID(t *testing.T) {
	tests := []struct {
		name         string
		setupPod     *corev1.Pod
		redisCluster *redisv1beta2.RedisCluster
		expectedID   int
		expectError  bool
	}{
		{
			name: "Successful retrieval of leader container",
			setupPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster-leader-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "redis-cluster-leader",
						},
						{
							Name: "another-container",
						},
					},
				},
			},
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
			},
			expectedID:  0,
			expectError: false,
		},
		{
			name:     "Pod not found",
			setupPod: &corev1.Pod{},
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
			},
			expectedID:  -1,
			expectError: true,
		},
		{
			name: "Leader container not found in the pod",
			setupPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster-leader-0",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "non-leader-container",
						},
					},
				},
			},
			redisCluster: &redisv1beta2.RedisCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-cluster",
					Namespace: "default",
				},
			},
			expectedID:  -1,
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := k8sClientFake.NewSimpleClientset(test.setupPod)
			logger := testr.New(t)
			id, pod := getContainerID(client, logger, test.redisCluster, test.setupPod.Name)
			if test.expectError {
				assert.Nil(t, pod, "Expected no pod but got one")
				assert.Equal(t, test.expectedID, id, "Expected ID does not match")
			} else {
				assert.NotNil(t, pod, "Expected a pod but got none")
				assert.Equal(t, test.expectedID, id, "Expected ID does not match")
				assert.Equal(t, test.setupPod.Name, pod.GetName(), "Pod names do not match")
				assert.Equal(t, test.setupPod.Namespace, pod.GetNamespace(), "Pod namespaces do not match")
			}
		})
	}
}

func Test_checkAttachedSlave(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name               string
		podName            string
		infoReturn         string
		infoErr            error
		expectedSlaveCount int
	}{
		{
			name:    "no attached slaves",
			podName: "pod1",
			infoReturn: "# Replication\r\n" +
				"role:master\r\n" +
				"connected_slaves:0\r\n" +
				"master_failover_state:no-failover\r\n" +
				"master_replid:7b634a76ebb7d5f07007f1d5aec8abff8200704e\r\n" +
				"master_replid2:0000000000000000000000000000000000000000\r\n" +
				"master_repl_offset:0\r\n" +
				"second_repl_offset:-1\r\n" +
				"repl_backlog_active:0\r\n" +
				"repl_backlog_size:1048576\r\n" +
				"repl_backlog_first_byte_offset:0\r\n" +
				"repl_backlog_histlen:0\r\n",
			expectedSlaveCount: 0,
		},
		{
			name:    "two attached slaves",
			podName: "pod2",
			infoReturn: "# Replication\r\n" +
				"role:master\r\n" +
				"connected_slaves:2\r\n" +
				"master_failover_state:no-failover\r\n" +
				"master_replid:7b634a76ebb7d5f07007f1d5aec8abff8200704e\r\n" +
				"master_replid2:0000000000000000000000000000000000000000\r\n" +
				"master_repl_offset:0\r\n" +
				"second_repl_offset:-1\r\n" +
				"repl_backlog_active:0\r\n" +
				"repl_backlog_size:1048576\r\n" +
				"repl_backlog_first_byte_offset:0\r\n" +
				"repl_backlog_histlen:0\r\n",
			expectedSlaveCount: 2,
		},
		{
			name:               "error fetching slave info",
			podName:            "pod3",
			infoReturn:         "",
			infoErr:            redis.ErrClosed,
			expectedSlaveCount: -1,
		},
		{
			name:    "field not found",
			podName: "pod2",
			infoReturn: "# Replication\r\n" +
				"role:master\r\n" +
				"master_failover_state:no-failover\r\n" +
				"master_replid:7b634a76ebb7d5f07007f1d5aec8abff8200704e\r\n" +
				"master_replid2:0000000000000000000000000000000000000000\r\n" +
				"master_repl_offset:0\r\n" +
				"second_repl_offset:-1\r\n" +
				"repl_backlog_active:0\r\n" +
				"repl_backlog_size:1048576\r\n" +
				"repl_backlog_first_byte_offset:0\r\n" +
				"repl_backlog_histlen:0\r\n",
			expectedSlaveCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			client, mock := redismock.NewClientMock()

			if tt.infoErr != nil {
				mock.ExpectInfo("Replication").SetErr(tt.infoErr)
			} else {
				mock.ExpectInfo("Replication").SetVal(tt.infoReturn)
			}

			slaveCount := checkAttachedSlave(ctx, client, logger, tt.podName)
			assert.Equal(t, tt.expectedSlaveCount, slaveCount, "Test case: "+tt.name)
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unmet expectations: %s", err)
			}
		})
	}
}

func Test_checkRedisServerRole(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name           string
		podName        string
		infoReturn     string
		infoErr        error
		shouldFail     bool
		expectedResult string
	}{
		{
			name:    "redis master role",
			podName: "pod1",
			infoReturn: "# Replication\r\n" +
				"role:master\r\n" +
				"connected_slaves:0\r\n" +
				"master_failover_state:no-failover\r\n" +
				"master_replid:7b634a76ebb7d5f07007f1d5aec8abff8200704e\r\n" +
				"master_replid2:0000000000000000000000000000000000000000\r\n" +
				"master_repl_offset:0\r\n" +
				"second_repl_offset:-1\r\n" +
				"repl_backlog_active:0\r\n" +
				"repl_backlog_size:1048576\r\n" +
				"repl_backlog_first_byte_offset:0\r\n" +
				"repl_backlog_histlen:0\r\n",
			expectedResult: "master",
		},
		{
			name:    "redis slave role",
			podName: "pod2",
			infoReturn: "# Replication\r\n" +
				"role:slave\r\n" +
				"connected_slaves:0\r\n" +
				"master_failover_state:no-failover\r\n" +
				"master_replid:7b634a76ebb7d5f07007f1d5aec8abff8200704e\r\n" +
				"master_replid2:0000000000000000000000000000000000000000\r\n" +
				"master_repl_offset:0\r\n" +
				"second_repl_offset:-1\r\n" +
				"repl_backlog_active:0\r\n" +
				"repl_backlog_size:1048576\r\n" +
				"repl_backlog_first_byte_offset:0\r\n" +
				"repl_backlog_histlen:0\r\n",
			expectedResult: "slave",
		},
		{
			name:       "error fetching role info",
			podName:    "pod3",
			infoErr:    redis.ErrClosed,
			shouldFail: true,
		},
		{
			name:    "field not found",
			podName: "pod2",
			infoReturn: "# Replication\r\n" +
				"connected_slaves:0\r\n" +
				"master_failover_state:no-failover\r\n" +
				"master_replid:7b634a76ebb7d5f07007f1d5aec8abff8200704e\r\n" +
				"master_replid2:0000000000000000000000000000000000000000\r\n" +
				"master_repl_offset:0\r\n" +
				"second_repl_offset:-1\r\n" +
				"repl_backlog_active:0\r\n" +
				"repl_backlog_size:1048576\r\n" +
				"repl_backlog_first_byte_offset:0\r\n" +
				"repl_backlog_histlen:0\r\n",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			client, mock := redismock.NewClientMock()

			if tt.infoErr != nil {
				mock.ExpectInfo("Replication").SetErr(tt.infoErr)
			} else {
				mock.ExpectInfo("Replication").SetVal(tt.infoReturn)
			}

			role := checkRedisServerRole(ctx, client, logger, tt.podName)
			if tt.shouldFail {
				assert.Empty(t, role, "Test case: "+tt.name)
			} else {
				assert.Equal(t, tt.expectedResult, role, "Test case: "+tt.name)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unmet expectations: %s", err)
			}
		})
	}
}

func TestCheckRedisCluster(t *testing.T) {
	logger := logr.Discard() // Discard logs

	tests := []struct {
		name               string
		expectError        error
		clusterNodesOutput string
		expectedResult     [][]string
	}{
		{
			name: "Detailed cluster nodes output",
			clusterNodesOutput: `07c37dfeb235213a872192d90877d0cd55635b91 127.0.0.1:30004@31004,hostname4 slave e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca 0 1426238317239 4 connected
67ed2db8d677e59ec4a4cefb06858cf2a1a89fa1 127.0.0.1:30002@31002,hostname2 master - 0 1426238316232 2 connected 5461-10922
292f8b365bb7edb5e285caf0b7e6ddc7265d2f4f 127.0.0.1:30003@31003,hostname3 master - 0 1426238318243 3 connected 10923-16383
6ec23923021cf3ffec47632106199cb7f496ce01 127.0.0.1:30005@31005,hostname5 slave 67ed2db8d677e59ec4a4cefb06858cf2a1a89fa1 0 1426238316232 5 connected
824fe116063bc5fcf9f4ffd895bc17aee7731ac3 127.0.0.1:30006@31006,hostname6 slave 292f8b365bb7edb5e285caf0b7e6ddc7265d2f4f 0 1426238317741 6 connected
e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca 127.0.0.1:30001@31001,hostname1 myself,master - 0 0 1 connected 0-5460`,
			expectedResult: [][]string{
				{"07c37dfeb235213a872192d90877d0cd55635b91", "127.0.0.1:30004@31004,hostname4", "slave", "e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca", "0", "1426238317239", "4", "connected"},
				{"67ed2db8d677e59ec4a4cefb06858cf2a1a89fa1", "127.0.0.1:30002@31002,hostname2", "master", "-", "0", "1426238316232", "2", "connected", "5461-10922"},
				{"292f8b365bb7edb5e285caf0b7e6ddc7265d2f4f", "127.0.0.1:30003@31003,hostname3", "master", "-", "0", "1426238318243", "3", "connected", "10923-16383"},
				{"6ec23923021cf3ffec47632106199cb7f496ce01", "127.0.0.1:30005@31005,hostname5", "slave", "67ed2db8d677e59ec4a4cefb06858cf2a1a89fa1", "0", "1426238316232", "5", "connected"},
				{"824fe116063bc5fcf9f4ffd895bc17aee7731ac3", "127.0.0.1:30006@31006,hostname6", "slave", "292f8b365bb7edb5e285caf0b7e6ddc7265d2f4f", "0", "1426238317741", "6", "connected"},
				{"e7d1eecce10fd6bb5eb35b9f99a514335d9ba9ca", "127.0.0.1:30001@31001,hostname1", "myself,master", "-", "0", "0", "1", "connected", "0-5460"},
			},
		},
		{
			name:           "Error from ClusterNodes",
			expectError:    redis.ErrClosed,
			expectedResult: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := redismock.NewClientMock()

			if tc.expectError != nil {
				mock.ExpectClusterNodes().SetErr(tc.expectError)
			} else {
				mock.ExpectClusterNodes().SetVal(tc.clusterNodesOutput)
			}
			result := checkRedisCluster(context.TODO(), db, logger)

			if tc.expectError != nil {
				assert.Nil(t, result)
			} else {
				assert.ElementsMatch(t, tc.expectedResult, result)
			}

			// Ensure all expectations are met
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})
	}
}
