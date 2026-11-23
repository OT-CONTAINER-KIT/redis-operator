package redisreplication

import (
	"context"
	"testing"

	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMissingMonitorCannotReturnAnEstablishedVoterToInitialization(t *testing.T) {
	for _, tc := range []struct {
		name, ownership, startup string
		reports                  int
		allowed                  bool
	}{
		{"first unattached generation", "", "", 0, true},
		{"bootstrap before handoff", "bootstrap", "", 0, true},
		{"all monitors lost after handoff", "sentinel", "", 0, false},
		{"one monitor lost after handoff", "sentinel", "enabled", 2, false},
		{"historical startup without ownership annotation", "", "enabled", 0, false},
		{"partial preexisting generation", "", "", 1, false},
		{"ambiguous partial initialization", "sentinel", "", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rr := &rrvb2.RedisReplication{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{common.SentinelOwnershipAnnotation: tc.ownership, common.SentinelStartupAnnotation: tc.startup}}}
			require.Equal(t, tc.allowed, initialMonitorInitializationAllowed(rr, tc.reports))
		})
	}
}

func TestCachedUnownedStateCannotAuthorizeMonitorRecreation(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, rrvb2.AddToScheme(scheme))
	cached := &rrvb2.RedisReplication{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "redis", UID: "owner", Generation: 1}}
	live := cached.DeepCopy()
	live.Annotations = map[string]string{common.SentinelOwnershipAnnotation: "sentinel"}
	r := &Reconciler{APIReader: clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(live).Build()}
	allowed, err := r.mayInitializeMissingMonitors(context.Background(), cached, 0)
	require.NoError(t, err)
	require.False(t, allowed)
	r.APIReader = nil
	allowed, err = r.mayInitializeMissingMonitors(context.Background(), cached, 0)
	require.NoError(t, err)
	require.False(t, allowed)
}
