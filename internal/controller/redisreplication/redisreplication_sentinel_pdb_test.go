package redisreplication

import (
	"context"
	"testing"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api/common/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewSentinelPodDisruptionBudget(t *testing.T) {
	tests := []struct {
		name               string
		size               int32
		params             commonapi.RedisPodDisruptionBudget
		wantMaxUnavailable *intstr.IntOrString
		wantMinAvailable   *intstr.IntOrString
	}{
		{
			name:             "defaults to a majority of three",
			size:             3,
			params:           commonapi.RedisPodDisruptionBudget{Enabled: true},
			wantMinAvailable: ptr.To(intstr.FromInt32(2)),
		},
		{
			name:             "defaults to a majority of five",
			size:             5,
			params:           commonapi.RedisPodDisruptionBudget{Enabled: true},
			wantMinAvailable: ptr.To(intstr.FromInt32(3)),
		},
		{
			name:             "minAvailable",
			size:             3,
			params:           commonapi.RedisPodDisruptionBudget{Enabled: true, MinAvailable: ptr.To(int32(1))},
			wantMinAvailable: ptr.To(intstr.FromInt32(1)),
		},
		{
			name:               "maxUnavailable wins over minAvailable",
			size:               3,
			params:             commonapi.RedisPodDisruptionBudget{Enabled: true, MinAvailable: ptr.To(int32(2)), MaxUnavailable: ptr.To(int32(1))},
			wantMaxUnavailable: ptr.To(intstr.FromInt32(1)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := newSentinelReplicationInstanceForTest()
			rr.Labels = map[string]string{"team": "cache"}
			rr.Spec.Sentinel.Size = tt.size
			rr.Spec.Sentinel.PodDisruptionBudget = &tt.params

			pdb := newSentinelPodDisruptionBudget(rr)

			assert.Equal(t, rr.SentinelStatefulSet(), pdb.Name)
			assert.Equal(t, rr.Namespace, pdb.Namespace)
			assert.Equal(t, map[string]string{"app": rr.SentinelStatefulSet(), "role": "sentinel"}, pdb.Spec.Selector.MatchLabels)
			assert.Equal(t, "cache", pdb.Labels["team"])
			assert.Equal(t, tt.wantMaxUnavailable, pdb.Spec.MaxUnavailable)
			assert.Equal(t, tt.wantMinAvailable, pdb.Spec.MinAvailable)
		})
	}
}

// The budget must select exactly the pods of the Sentinel StatefulSet, or it
// silently protects nothing.
func TestSentinelPodDisruptionBudgetSelectsSentinelPods(t *testing.T) {
	rr := newSentinelReplicationInstanceForTest()
	rr.Spec.Sentinel.PodDisruptionBudget = &commonapi.RedisPodDisruptionBudget{Enabled: true}

	pdb := newSentinelPodDisruptionBudget(rr)
	sts := newSentinelStatefulSet(rr, rr.SentinelHLService())

	selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
	require.NoError(t, err)
	assert.True(t, selector.Matches(labels.Set(sts.Spec.Template.Labels)))
}

func TestReconcileSentinelPodDisruptionBudget(t *testing.T) {
	require.NoError(t, rrvb2.AddToScheme(scheme.Scheme))

	enabled := func(rr *rrvb2.RedisReplication) {
		rr.Spec.Sentinel.PodDisruptionBudget = &commonapi.RedisPodDisruptionBudget{Enabled: true, MaxUnavailable: ptr.To(int32(1))}
	}
	tests := []struct {
		name       string
		configure  func(*rrvb2.RedisReplication)
		wantExists bool
	}{
		{name: "enabled creates the budget", configure: enabled, wantExists: true},
		{name: "unset removes the budget", configure: func(rr *rrvb2.RedisReplication) {
			rr.Spec.Sentinel.PodDisruptionBudget = nil
		}},
		{name: "disabled removes the budget", configure: func(rr *rrvb2.RedisReplication) {
			rr.Spec.Sentinel.PodDisruptionBudget = &commonapi.RedisPodDisruptionBudget{Enabled: false}
		}},
		{name: "turning Sentinel off removes the budget", configure: func(rr *rrvb2.RedisReplication) {
			enabled(rr)
			rr.Spec.Sentinel.Size = 0
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := newSentinelReplicationInstanceForTest()
			rr.UID = "rr-uid"
			cli := clientfake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(rr.DeepCopy()).Build()
			r := &Reconciler{Client: cli}

			// Start every case from a budget the operator already created.
			enabled(rr)
			require.NoError(t, r.reconcileSentinelPodDisruptionBudget(context.Background(), rr))

			tt.configure(rr)
			require.NoError(t, r.reconcileSentinelPodDisruptionBudget(context.Background(), rr))

			got := &policyv1.PodDisruptionBudget{}
			err := cli.Get(context.Background(), types.NamespacedName{Namespace: rr.Namespace, Name: rr.SentinelStatefulSet()}, got)
			if tt.wantExists {
				require.NoError(t, err)
				assert.Equal(t, ptr.To(intstr.FromInt32(1)), got.Spec.MaxUnavailable)
				require.NotNil(t, metav1.GetControllerOf(got))
				assert.Equal(t, rr.UID, metav1.GetControllerOf(got).UID)
			} else {
				assert.True(t, apierrors.IsNotFound(err), "expected no budget, got %v", err)
			}
		})
	}
}
