package k8sutils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

// fakeRedisServer speaks enough RESP for go-redis v9 (RESP3 handshake
// included): HELLO, PING and CLUSTER NODES. Everything else is rejected, so a
// test that expects no other traffic will notice.
type fakeRedisServer struct {
	ln           net.Listener
	port         int
	nodesPayload string
	pongReply    string

	mu    sync.Mutex
	pings int
}

func newFakeRedisServer(t *testing.T, nodesPayload string) *fakeRedisServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start fake redis server: %v", err)
	}
	s := &fakeRedisServer{ln: ln, port: ln.Addr().(*net.TCPAddr).Port, nodesPayload: nodesPayload, pongReply: "+PONG\r\n"}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *fakeRedisServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeRedisServer) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		args, err := readRESPArray(r)
		if err != nil {
			return
		}
		if len(args) == 0 {
			continue
		}
		switch cmd := strings.ToUpper(args[0]); {
		case cmd == "HELLO":
			// Minimal RESP3 map reply; the handshake only needs valid framing.
			fmt.Fprint(conn, "%2\r\n$6\r\nserver\r\n$5\r\nredis\r\n$5\r\nproto\r\n:3\r\n")
		case cmd == "PING":
			s.mu.Lock()
			s.pings++
			s.mu.Unlock()
			fmt.Fprint(conn, s.pongReply)
		case cmd == "CLUSTER" && len(args) > 1 && strings.ToUpper(args[1]) == "NODES":
			fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(s.nodesPayload), s.nodesPayload)
		default:
			fmt.Fprint(conn, "-ERR unsupported command\r\n")
		}
	}
}

func (s *fakeRedisServer) pingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pings
}

func readRESPArray(r *bufio.Reader) ([]string, error) {
	header, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	header = strings.TrimRight(header, "\r\n")
	if !strings.HasPrefix(header, "*") {
		return nil, fmt.Errorf("unexpected line %q", header)
	}
	n, err := strconv.Atoi(header[1:])
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, n)
	for i := 0; i < n; i++ {
		lenLine, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		lenLine = strings.TrimRight(lenLine, "\r\n")
		if !strings.HasPrefix(lenLine, "$") {
			return nil, fmt.Errorf("expected bulk string, got %q", lenLine)
		}
		bl, err := strconv.Atoi(lenLine[1:])
		if err != nil {
			return nil, err
		}
		buf := make([]byte, bl+2) // payload + CRLF
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args = append(args, string(buf[:bl]))
	}
	return args, nil
}

// replicationCR builds a RedisCluster with independent leader/follower counts.
func replicationCR(name string, leaders, followers, port int) *rcvb2.RedisCluster {
	cr := &rcvb2.RedisCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: rcvb2.RedisClusterSpec{
			Port:           ptr.To(port),
			ClusterVersion: ptr.To("v7"),
		},
	}
	cr.Spec.RedisLeader.Replicas = ptr.To(int32(leaders))
	cr.Spec.RedisFollower.Replicas = ptr.To(int32(followers))
	return cr
}

// replicationPods creates leader/follower pods. Unset IP funcs default to
// 127.0.0.1, where the fake redis server listens; 127.0.0.2 listens on
// nothing and fails every dial immediately with ECONNREFUSED.
func replicationPods(name string, leaders, followers int, leaderIP, followerIP func(i int) string) []runtime.Object {
	defaultIP := func(int) string { return "127.0.0.1" }
	if leaderIP == nil {
		leaderIP = defaultIP
	}
	if followerIP == nil {
		followerIP = defaultIP
	}
	pods := []runtime.Object{}
	for i := 0; i < leaders; i++ {
		pods = append(pods, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-leader-%d", name, i), Namespace: "default"},
			Status:     corev1.PodStatus{PodIP: leaderIP(i)},
		})
	}
	for i := 0; i < followers; i++ {
		pods = append(pods, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-follower-%d", name, i), Namespace: "default"},
			Status:     corev1.PodStatus{PodIP: followerIP(i)},
		})
	}
	return pods
}

func TestExecuteRedisReplicationCommand(t *testing.T) {
	const name = "redis-cluster"

	t.Run("adds every follower round-robin when none are in the cluster", func(t *testing.T) {
		// Leader lines use 127.0.0.9 so follower IPs (127.0.0.1) never match.
		server := newFakeRedisServer(t, "id0 127.0.0.9:6379@16379 master - 0 0 1 connected 0-16383\n")
		client := k8sClientFake.NewSimpleClientset(replicationPods(name, 2, 3, nil, nil)...)

		var execCmds [][]string
		err := executeRedisReplicationCommand(context.Background(), client,
			replicationCR(name, 2, 3, server.port),
			func(_ context.Context, _ kubernetes.Interface, _ *rcvb2.RedisCluster, cmd []string, _ string) error {
				execCmds = append(execCmds, cmd)
				return nil
			})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if len(execCmds) != 3 {
			t.Fatalf("expected 3 add-node executions, got %d", len(execCmds))
		}
		for i, cmd := range execCmds {
			joined := strings.Join(cmd, " ")
			wantLeader := fmt.Sprintf("%s-leader-%d", name, i%2)
			if !strings.Contains(joined, fmt.Sprintf("%s-follower-%d", name, i)) ||
				!strings.Contains(joined, wantLeader) {
				t.Errorf("execution %d: expected follower-%d paired with %s, got %q", i, i, wantLeader, joined)
			}
		}
		if server.pingCount() != 3 {
			t.Errorf("expected 3 pings (one per follower), got %d", server.pingCount())
		}
	})

	t.Run("returns without exec when no followers are configured", func(t *testing.T) {
		server := newFakeRedisServer(t, "")
		client := k8sClientFake.NewSimpleClientset(replicationPods(name, 2, 0, nil, nil)...)

		execCount := 0
		err := executeRedisReplicationCommand(context.Background(), client,
			replicationCR(name, 2, 0, server.port),
			func(_ context.Context, _ kubernetes.Interface, _ *rcvb2.RedisCluster, _ []string, _ string) error {
				execCount++
				return nil
			})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if execCount != 0 {
			t.Errorf("expected no executions with followers=0, got %d", execCount)
		}
	})

	t.Run("handles fewer followers than leaders", func(t *testing.T) {
		// followers(1) < leaders(3) used to spin forever: followerPerLeader
		// truncated to 0 and followerIdx never advanced.
		server := newFakeRedisServer(t, "id0 127.0.0.9:6379@16379 master - 0 0 1 connected 0-16383\n")
		client := k8sClientFake.NewSimpleClientset(replicationPods(name, 3, 1, nil, nil)...)

		var execCmds [][]string
		err := executeRedisReplicationCommand(context.Background(), client,
			replicationCR(name, 3, 1, server.port),
			func(_ context.Context, _ kubernetes.Interface, _ *rcvb2.RedisCluster, cmd []string, _ string) error {
				execCmds = append(execCmds, cmd)
				return nil
			})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if len(execCmds) != 1 {
			t.Fatalf("expected 1 add-node execution, got %d", len(execCmds))
		}
		joined := strings.Join(execCmds[0], " ")
		if !strings.Contains(joined, name+"-follower-0") || !strings.Contains(joined, name+"-leader-0") {
			t.Errorf("expected follower-0 paired with leader-0, got %q", joined)
		}
	})

	t.Run("skips followers that are already part of the cluster", func(t *testing.T) {
		// follower-0 (127.0.0.10) is listed in CLUSTER NODES, follower-1 is not.
		payload := "id0 127.0.0.9:6379@16379 master - 0 0 1 connected 0-16383\n" +
			"id1 127.0.0.10:6379@16379 slave id0 0 0 2 connected\n"
		server := newFakeRedisServer(t, payload)
		client := k8sClientFake.NewSimpleClientset(replicationPods(name, 1, 2,
			nil,
			func(i int) string {
				if i == 0 {
					return "127.0.0.10"
				}
				return "127.0.0.1"
			})...)

		var execCmds [][]string
		err := executeRedisReplicationCommand(context.Background(), client,
			replicationCR(name, 1, 2, server.port),
			func(_ context.Context, _ kubernetes.Interface, _ *rcvb2.RedisCluster, cmd []string, _ string) error {
				execCmds = append(execCmds, cmd)
				return nil
			})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if len(execCmds) != 1 {
			t.Fatalf("expected only the missing follower to be added, got %d executions", len(execCmds))
		}
		if !strings.Contains(strings.Join(execCmds[0], " "), name+"-follower-1") {
			t.Errorf("expected the execution to target follower-1, got %q", strings.Join(execCmds[0], " "))
		}
		if server.pingCount() != 1 {
			t.Errorf("expected 1 ping (present followers are skipped before ping), got %d", server.pingCount())
		}
	})

	t.Run("fails fast when a follower is unreachable", func(t *testing.T) {
		// follower-0 dials 127.0.0.2 where nothing listens; follower-1 would be
		// reachable but must never be attempted after the failure.
		server := newFakeRedisServer(t, "id0 127.0.0.9:6379@16379 master - 0 0 1 connected 0-16383\n")
		client := k8sClientFake.NewSimpleClientset(replicationPods(name, 3, 2,
			nil,
			func(i int) string {
				if i == 0 {
					return "127.0.0.2"
				}
				return "127.0.0.1"
			})...)

		execCount := 0
		err := executeRedisReplicationCommand(context.Background(), client,
			replicationCR(name, 3, 2, server.port),
			func(_ context.Context, _ kubernetes.Interface, _ *rcvb2.RedisCluster, _ []string, _ string) error {
				execCount++
				return nil
			})

		if err == nil {
			t.Fatal("expected an error for the unreachable follower, got nil")
		}
		if !strings.Contains(err.Error(), "failed to ping follower "+name+"-follower-0") {
			t.Errorf("error should name the unreachable follower, got %v", err)
		}
		if execCount != 0 {
			t.Errorf("fail-fast must not attempt followers after the failure, got %d executions", execCount)
		}
	})

	t.Run("surfaces a failed CLUSTER NODES lookup", func(t *testing.T) {
		// leader-0 dials 127.0.0.2 where nothing listens.
		server := newFakeRedisServer(t, "")
		client := k8sClientFake.NewSimpleClientset(replicationPods(name, 1, 1,
			func(int) string { return "127.0.0.2" }, nil)...)

		execCount := 0
		err := executeRedisReplicationCommand(context.Background(), client,
			replicationCR(name, 1, 1, server.port),
			func(_ context.Context, _ kubernetes.Interface, _ *rcvb2.RedisCluster, _ []string, _ string) error {
				execCount++
				return nil
			})

		if err == nil {
			t.Fatal("expected an error for the failed cluster nodes lookup, got nil")
		}
		if !strings.Contains(err.Error(), "failed to get cluster nodes") {
			t.Errorf("error should mention the cluster nodes lookup, got %v", err)
		}
		if execCount != 0 {
			t.Errorf("no add-node may run without cluster nodes, got %d executions", execCount)
		}
	})

	t.Run("propagates a failed add-node exec", func(t *testing.T) {
		server := newFakeRedisServer(t, "id0 127.0.0.9:6379@16379 master - 0 0 1 connected 0-16383\n")
		client := k8sClientFake.NewSimpleClientset(replicationPods(name, 1, 1, nil, nil)...)

		execCount := 0
		err := executeRedisReplicationCommand(context.Background(), client,
			replicationCR(name, 1, 1, server.port),
			func(_ context.Context, _ kubernetes.Interface, _ *rcvb2.RedisCluster, _ []string, _ string) error {
				execCount++
				return errors.New("exec failed")
			})

		if err == nil {
			t.Fatal("expected the add-node exec failure to surface, got nil")
		}
		if !strings.Contains(err.Error(), "failed to add follower "+name+"-follower-0") {
			t.Errorf("error should name the follower being added, got %v", err)
		}
		if execCount != 1 {
			t.Errorf("expected exactly one exec attempt, got %d", execCount)
		}
	})

	t.Run("skips execution when the ping reply is unexpected", func(t *testing.T) {
		server := newFakeRedisServer(t, "id0 127.0.0.9:6379@16379 master - 0 0 1 connected 0-16383\n")
		server.pongReply = "+OK\r\n"
		client := k8sClientFake.NewSimpleClientset(replicationPods(name, 1, 1, nil, nil)...)

		execCount := 0
		err := executeRedisReplicationCommand(context.Background(), client,
			replicationCR(name, 1, 1, server.port),
			func(_ context.Context, _ kubernetes.Interface, _ *rcvb2.RedisCluster, _ []string, _ string) error {
				execCount++
				return nil
			})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if execCount != 0 {
			t.Errorf("add-node must not run when ping does not answer PONG, got %d executions", execCount)
		}
	})
}
