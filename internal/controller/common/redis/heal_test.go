package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	common "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	redisservice "github.com/OT-CONTAINER-KIT/redis-operator/internal/service/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestUpdateRedisRoleLabelSkipsUnprobeablePods(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	clientset := k8sfake.NewSimpleClientset(
		newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true),
		newLabeledRedisPod("redis-1", labels, "", corev1.PodRunning, true),
		newLabeledRedisPod("redis-2", labels, "10.0.0.12", corev1.PodRunning, false),
		newLabeledRedisPod("redis-3", labels, "", corev1.PodPending, false),
	)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.10": true,
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.NoError(t, err)
	assert.Equal(t, []string{"10.0.0.10"}, redisClient.connectHosts)

	readyPod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, readyPod.Labels[common.RedisRoleLabelKey])

	for _, podName := range []string{"redis-1", "redis-2", "redis-3"} {
		pod, getErr := clientset.CoreV1().Pods("default").Get(context.Background(), podName, metav1.GetOptions{})
		require.NoError(t, getErr)
		assert.Empty(t, pod.Labels[common.RedisRoleLabelKey])
	}
}

func TestUpdateRedisRoleLabelRemovesStaleMasterLabelWhenAMasterWithReplicasIsConfirmed(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	staleMaster := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	staleMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	newMaster := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)

	clientset := k8sfake.NewSimpleClientset(staleMaster, newMaster)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.11": true,
		},
		replicasByHost: map[string]int{
			"10.0.0.11": 1,
		},
		errByHost: map[string]error{
			"10.0.0.10": dialError(),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.NoError(t, err)

	unreachablePod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, unreachablePod.Labels, common.RedisRoleLabelKey)

	masterPod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, masterPod.Labels[common.RedisRoleLabelKey])

	// A merge patch with a null value is idempotent, unlike a JSON Patch "remove"
	// op, which returns 422 once the label is gone.
	var removePatches int
	for _, action := range clientset.Actions() {
		patchAction, ok := action.(k8stesting.PatchAction)
		if !ok || patchAction.GetName() != "redis-0" {
			continue
		}
		removePatches++
		assert.Equal(t, types.MergePatchType, patchAction.GetPatchType())
		assert.JSONEq(t, `{"metadata":{"labels":{"redis-role":null}}}`, string(patchAction.GetPatch()))
	}
	assert.Equal(t, 1, removePatches)
}

func TestUpdateRedisRoleLabelKeepsStaleMasterLabelWhenNoMasterIsConfirmed(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	unreachableMaster := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	unreachableMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	healthySlave := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)

	clientset := k8sfake.NewSimpleClientset(unreachableMaster, healthySlave)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.11": false,
		},
		errByHost: map[string]error{
			"10.0.0.10": dialError(),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.NoError(t, err)

	unreachablePod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, unreachablePod.Labels[common.RedisRoleLabelKey])

	healthyPod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelSlave, healthyPod.Labels[common.RedisRoleLabelKey])
}

// An operator-side fault - a rotated password, a TLS change, a network policy -
// fails every probe while the pods stay Ready and keep serving. Stripping their
// labels would take the master and replica services to zero endpoints.
func TestUpdateRedisRoleLabelKeepsRoleLabelsWhenEveryProbeFails(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	master := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	master.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	slave := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)
	slave.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelSlave

	clientset := k8sfake.NewSimpleClientset(master, slave)
	redisClient := &fakeRedisClient{
		errByHost: map[string]error{
			"10.0.0.10": redisServerError("WRONGPASS invalid username-password pair"),
			"10.0.0.11": redisServerError("WRONGPASS invalid username-password pair"),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.NoError(t, err)

	for podName, role := range map[string]string{
		"redis-0": common.RedisRoleLabelMaster,
		"redis-1": common.RedisRoleLabelSlave,
	} {
		pod, getErr := clientset.CoreV1().Pods("default").Get(context.Background(), podName, metav1.GetOptions{})
		require.NoError(t, getErr)
		assert.Equal(t, role, pod.Labels[common.RedisRoleLabelKey])
	}
}

// A stale slave label is never removed: doing so on an operator-side fault would
// empty the replica service, and a genuinely dead pod leaves that service once
// the node lifecycle controller marks it NotReady.
func TestUpdateRedisRoleLabelKeepsSlaveLabelOnUnreachablePod(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	unreachableSlave := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	unreachableSlave.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelSlave
	master := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)
	master.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster

	clientset := k8sfake.NewSimpleClientset(unreachableSlave, master)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.11": true,
		},
		replicasByHost: map[string]int{
			"10.0.0.11": 1,
		},
		errByHost: map[string]error{
			"10.0.0.10": dialError(),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.NoError(t, err)

	unreachablePod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelSlave, unreachablePod.Labels[common.RedisRoleLabelKey])
}

// The pods this branch exists for are the ones most likely to be deleted between
// the List and the Patch. A deleted pod took its label with it, so it is passed
// over without abandoning the rest and without failing the reconcile.
func TestUpdateRedisRoleLabelPassesOverPodDeletedBeforeStaleLabelRemoval(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	goneMaster := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	goneMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	staleMaster := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)
	staleMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	newMaster := newLabeledRedisPod("redis-2", labels, "10.0.0.12", corev1.PodRunning, true)

	clientset := k8sfake.NewSimpleClientset(goneMaster, staleMaster, newMaster)
	clientset.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.(k8stesting.PatchAction).GetName() == "redis-0" {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, "redis-0")
		}
		return false, nil, nil
	})
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.12": true,
		},
		replicasByHost: map[string]int{
			"10.0.0.12": 1,
		},
		errByHost: map[string]error{
			"10.0.0.10": dialError(),
			"10.0.0.11": dialError(),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.NoError(t, err)

	stillLabelled, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, stillLabelled.Labels, common.RedisRoleLabelKey)
}

// Any other failure leaves a dead pod behind the master service, so it must fail
// the reconcile - after the remaining removals have still been attempted.
func TestUpdateRedisRoleLabelReturnsErrorWhenStaleLabelRemovalFails(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	forbiddenMaster := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	forbiddenMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	staleMaster := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)
	staleMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	newMaster := newLabeledRedisPod("redis-2", labels, "10.0.0.12", corev1.PodRunning, true)

	clientset := k8sfake.NewSimpleClientset(forbiddenMaster, staleMaster, newMaster)
	clientset.PrependReactor("patch", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.(k8stesting.PatchAction).GetName() == "redis-0" {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "redis-0", errors.New("patch denied"))
		}
		return false, nil, nil
	})
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.12": true,
		},
		replicasByHost: map[string]int{
			"10.0.0.12": 1,
		},
		errByHost: map[string]error{
			"10.0.0.10": dialError(),
			"10.0.0.11": dialError(),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.Error(t, err)
	assert.True(t, apierrors.IsForbidden(err), "expected the API error to be preserved, got %v", err)
	assert.ErrorContains(t, err, "redis-0")

	stillForbidden, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, stillForbidden.Labels[common.RedisRoleLabelKey])

	removed, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, removed.Labels, common.RedisRoleLabelKey)

	masterPod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-2", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, masterPod.Labels[common.RedisRoleLabelKey])
}

// dialError is what an unreachable pod actually produces: a dial failure or, for
// a blackholed node, a timeout. Both satisfy net.Error.
func dialError() error {
	return &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}
}

// netDialTimeoutError returns the error net.Dialer reports for its own dial
// timeout, obtained deterministically by dialing with an already-expired
// deadline. Its inner error satisfies errors.Is(err, context.DeadlineExceeded)
// even though no caller context expired, which is what makes it worth pinning.
func netDialTimeoutError(t *testing.T) error {
	t.Helper()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err := (&net.Dialer{}).DialContext(ctx, "tcp", "127.0.0.1:1")
	require.Error(t, err)
	var opErr *net.OpError
	require.True(t, errors.As(err, &opErr), "expected *net.OpError, got %T", err)
	require.Equal(t, "dial", opErr.Op)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	return err
}

// redisServerError stands in for a reply from a reachable Redis, which is what
// go-redis returns for WRONGPASS/NOAUTH/NOPERM/LOADING.
type redisServerError string

func (e redisServerError) Error() string { return string(e) }

func (e redisServerError) RedisError() {}

// An auth failure means the pod answered, so it is not evidence that the pod is
// unreachable - even when another pod does report itself as master.
func TestUpdateRedisRoleLabelKeepsMasterLabelOnAuthFailure(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	staleMaster := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	staleMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	otherMaster := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)

	clientset := k8sfake.NewSimpleClientset(staleMaster, otherMaster)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.11": true,
		},
		replicasByHost: map[string]int{
			"10.0.0.11": 1,
		},
		errByHost: map[string]error{
			"10.0.0.10": redisServerError("WRONGPASS invalid username-password pair"),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.NoError(t, err)

	unreachablePod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, unreachablePod.Labels[common.RedisRoleLabelKey])
}

// A replica pod recreated while the master is unreachable boots as a standalone
// master, because replication is only attached at runtime. It answers as master
// but has no replicas, so it is not evidence that the unreachable master was
// replaced; stripping the label would route the master service to an empty pod.
func TestUpdateRedisRoleLabelKeepsMasterLabelWhenOnlyAStandaloneMasterAnswers(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	unreachableMaster := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	unreachableMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	standalone := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)
	standalone.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelSlave

	clientset := k8sfake.NewSimpleClientset(unreachableMaster, standalone)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.11": true,
		},
		errByHost: map[string]error{
			"10.0.0.10": dialError(),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.NoError(t, err)
	// One connection per pod: the replica count is read on the handle that
	// answered the role probe.
	assert.Equal(t, []string{"10.0.0.10", "10.0.0.11"}, redisClient.connectHosts)

	unreachablePod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, unreachablePod.Labels[common.RedisRoleLabelKey])

	standalonePod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, standalonePod.Labels[common.RedisRoleLabelKey])
}

// After a two-pod failover the promoted survivor has no replica left to attach,
// so it cannot prove itself here. The controller has confirmed it through
// Sentinel, and passes it in; the lost master's label is then stale and removed.
func TestUpdateRedisRoleLabelRemovesStaleMasterLabelWhenTheIdentifiedMasterHasNoReplicas(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	lostMaster := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	lostMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	survivor := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)
	survivor.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelSlave

	clientset := k8sfake.NewSimpleClientset(lostMaster, survivor)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.11": true,
		},
		errByHost: map[string]error{
			"10.0.0.10": dialError(),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "redis-1")

	require.NoError(t, err)

	lostPod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotContains(t, lostPod.Labels, common.RedisRoleLabelKey)

	survivorPod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-1", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, survivorPod.Labels[common.RedisRoleLabelKey])
}

// The identified master only counts once it answers as master here; naming a
// pod that cannot be reached changes nothing.
func TestUpdateRedisRoleLabelKeepsMasterLabelWhenTheIdentifiedMasterDoesNotAnswer(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	unreachableMaster := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	unreachableMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	standalone := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)

	clientset := k8sfake.NewSimpleClientset(unreachableMaster, standalone)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.11": true,
		},
		errByHost: map[string]error{
			"10.0.0.10": dialError(),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "redis-0")

	require.NoError(t, err)

	unreachablePod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, unreachablePod.Labels[common.RedisRoleLabelKey])
}

// A master whose replica count cannot be read is not evidence either way.
func TestUpdateRedisRoleLabelKeepsMasterLabelWhenReplicaCountCannotBeRead(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	unreachableMaster := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	unreachableMaster.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	otherMaster := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)

	clientset := k8sfake.NewSimpleClientset(unreachableMaster, otherMaster)
	redisClient := &fakeRedisClient{
		isMasterByHost: map[string]bool{
			"10.0.0.11": true,
		},
		replicaErrByHost: map[string]error{
			"10.0.0.11": errors.New("read tcp: i/o timeout"),
		},
		errByHost: map[string]error{
			"10.0.0.10": dialError(),
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}

	err := h.UpdateRedisRoleLabel(context.Background(), "default", labels, nil, nil, "")

	require.NoError(t, err)

	unreachablePod, err := clientset.CoreV1().Pods("default").Get(context.Background(), "redis-0", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.RedisRoleLabelMaster, unreachablePod.Labels[common.RedisRoleLabelKey])
}

// A cancelled reconcile - a manager shutdown, say - fails every probe. That is
// not evidence about any pod, so the sweep must abort and report it rather than
// deciding labels from a view it could not build.
func TestUpdateRedisRoleLabelPropagatesContextCancellation(t *testing.T) {
	labels := map[string]string{"app": "redis"}
	master := newLabeledRedisPod("redis-0", labels, "10.0.0.10", corev1.PodRunning, true)
	master.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelMaster
	slave := newLabeledRedisPod("redis-1", labels, "10.0.0.11", corev1.PodRunning, true)
	slave.Labels[common.RedisRoleLabelKey] = common.RedisRoleLabelSlave

	clientset := k8sfake.NewSimpleClientset(master, slave)
	ctx, cancel := context.WithCancel(context.Background())
	redisClient := &fakeRedisClient{
		errByHost: map[string]error{
			"10.0.0.10": context.Canceled,
			"10.0.0.11": context.Canceled,
		},
	}
	h := &healer{
		k8s:   clientset,
		redis: redisClient,
	}
	cancel()

	err := h.UpdateRedisRoleLabel(ctx, "default", labels, nil, nil, "")

	require.ErrorIs(t, err, context.Canceled)

	for podName, role := range map[string]string{
		"redis-0": common.RedisRoleLabelMaster,
		"redis-1": common.RedisRoleLabelSlave,
	} {
		pod, getErr := clientset.CoreV1().Pods("default").Get(context.Background(), podName, metav1.GetOptions{})
		require.NoError(t, getErr)
		assert.Equal(t, role, pod.Labels[common.RedisRoleLabelKey])
	}
}

func TestIsConnectivityError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"dial failure", dialError(), true},
		{"timeout", &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}, true},
		// net.Dialer reports its own dial timeout with an inner error that
		// satisfies errors.Is(err, context.DeadlineExceeded); the OpError has to
		// win over the context check or a blackholed node is never recognised.
		{"dial timeout reported by net.Dialer", netDialTimeoutError(t), true},
		{"connection reset", &net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset by peer")}, true},
		{"dns failure", &net.OpError{Op: "dial", Net: "tcp", Err: &net.DNSError{Err: "no such host"}}, true},
		{"socket deadline", os.ErrDeadlineExceeded, true},
		{"eof", io.EOF, true},
		// Context errors belong to the reconcile, not the pod; the caller handles
		// them before any label decision is made.
		{"context deadline", context.DeadlineExceeded, false},
		{"context cancelled", context.Canceled, false},
		{"wrong password", redisServerError("WRONGPASS invalid username-password pair"), false},
		{"no auth", redisServerError("NOAUTH Authentication required."), false},
		{"loading", redisServerError("LOADING Redis is loading the dataset in memory"), false},
		{"wrapped server reply", fmt.Errorf("probe: %w", redisServerError("NOPERM")), false},
		{"unknown certificate authority", x509.UnknownAuthorityError{}, false},
		{"certificate verification", &tls.CertificateVerificationError{Err: errors.New("expired")}, false},
		// crypto/tls surfaces a TLS alert sent by the peer, and a handshake it
		// aborted itself, as *net.OpError; both mean the pod answered on the socket.
		{"remote tls alert", &net.OpError{Op: "remote error", Net: "tcp", Err: errors.New("tls: bad certificate")}, false},
		{"wrapped remote tls alert", fmt.Errorf("probe: %w", &net.OpError{Op: "remote error", Net: "tcp", Err: errors.New("tls: unknown certificate authority")}), false},
		{"local tls handshake abort", &net.OpError{Op: "local error", Net: "tcp", Err: errors.New("tls: handshake failure")}, false},
		{"unrecognised error is inconclusive", errors.New("something went wrong"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isConnectivityError(tt.err))
		})
	}
}

type fakeRedisClient struct {
	connectHosts     []string
	isMasterByHost   map[string]bool
	replicasByHost   map[string]int
	errByHost        map[string]error
	replicaErrByHost map[string]error
}

func (f *fakeRedisClient) Connect(info *redisservice.ConnectionInfo) redisservice.Service {
	f.connectHosts = append(f.connectHosts, info.Host)
	return &fakeRedisService{
		host:             info.Host,
		isMasterByHost:   f.isMasterByHost,
		replicasByHost:   f.replicasByHost,
		errByHost:        f.errByHost,
		replicaErrByHost: f.replicaErrByHost,
	}
}

type fakeRedisService struct {
	host             string
	isMasterByHost   map[string]bool
	replicasByHost   map[string]int
	errByHost        map[string]error
	replicaErrByHost map[string]error
}

func (f *fakeRedisService) IsMaster(context.Context) (bool, error) {
	if err := f.errByHost[f.host]; err != nil {
		return false, err
	}
	return f.isMasterByHost[f.host], nil
}

func (f *fakeRedisService) GetAttachedReplicaCount(context.Context) (int, error) {
	if err := f.replicaErrByHost[f.host]; err != nil {
		return 0, err
	}
	return f.replicasByHost[f.host], nil
}

func (f *fakeRedisService) SentinelMonitor(context.Context, *redisservice.ConnectionInfo, string, string) error {
	return nil
}

func (f *fakeRedisService) SentinelSet(context.Context, string, string, string) error {
	return nil
}

func (f *fakeRedisService) SentinelReset(context.Context, string) error {
	return nil
}

func (f *fakeRedisService) GetInfoSentinel(context.Context) (*redisservice.InfoSentinelResult, error) {
	return &redisservice.InfoSentinelResult{}, nil
}

func (f *fakeRedisService) GetClusterInfo(context.Context) (*redisservice.ClusterStatus, error) {
	return &redisservice.ClusterStatus{}, nil
}

func newLabeledRedisPod(name string, labels map[string]string, podIP string, phase corev1.PodPhase, ready bool) *corev1.Pod {
	podLabels := map[string]string{}
	for key, value := range labels {
		podLabels[key] = value
	}

	readyStatus := corev1.ConditionFalse
	if ready {
		readyStatus = corev1.ConditionTrue
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    podLabels,
		},
		Status: corev1.PodStatus{
			Phase: phase,
			PodIP: podIP,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: readyStatus,
				},
			},
		},
	}
}
