package poddisruptionbudget

import (
	"context"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/reconciler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	namespace = "default"
	name      = "redis-replication-s"
)

// Any namespaced kind registered in the client-go scheme can stand in for the
// RedisReplication: only its UID decides control.
var (
	owner = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "redis-replication", Namespace: namespace, UID: "owner-uid"}}
	other = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "redis-replication", Namespace: namespace, UID: "other-uid"}}
)

func selector(app string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: map[string]string{"app": app, "role": "sentinel"}}
}

func newPDB(maxUnavailable int32, controller *corev1.Secret) *policyv1.PodDisruptionBudget {
	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{"app": name}},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector:       selector(name),
			MaxUnavailable: ptr.To(intstr.FromInt32(maxUnavailable)),
		},
	}
	if controller != nil {
		if err := controllerutil.SetControllerReference(controller, pdb, scheme.Scheme); err != nil {
			panic(err)
		}
	}
	return pdb
}

func TestNeedUpdate(t *testing.T) {
	expected := newPDB(1, nil)
	tests := []struct {
		name   string
		mutate func(*policyv1.PodDisruptionBudget)
		want   bool
	}{
		{name: "identical", mutate: func(*policyv1.PodDisruptionBudget) {}, want: false},
		{name: "extra label", mutate: func(p *policyv1.PodDisruptionBudget) { p.Labels["extra"] = "x" }, want: false},
		{name: "missing label", mutate: func(p *policyv1.PodDisruptionBudget) { p.Labels = nil }, want: true},
		{name: "different selector", mutate: func(p *policyv1.PodDisruptionBudget) { p.Spec.Selector = selector("other") }, want: true},
		{name: "different maxUnavailable", mutate: func(p *policyv1.PodDisruptionBudget) {
			p.Spec.MaxUnavailable = ptr.To(intstr.FromInt32(2))
		}, want: true},
		{name: "minAvailable instead of maxUnavailable", mutate: func(p *policyv1.PodDisruptionBudget) {
			p.Spec.MaxUnavailable = nil
			p.Spec.MinAvailable = ptr.To(intstr.FromInt32(2))
		}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existed := expected.DeepCopy()
			tt.mutate(existed)
			assert.Equal(t, tt.want, needUpdate(expected, existed))
		})
	}
}

func TestReconcileOwnership(t *testing.T) {
	tests := []struct {
		name               string
		existing           *policyv1.PodDisruptionBudget
		wantErr            bool
		wantMaxUnavailable int32
		wantUID            types.UID
	}{
		{
			name:               "missing budget is created and controlled by the owner",
			wantMaxUnavailable: 1,
			wantUID:            owner.UID,
		},
		{
			name:               "budget controlled by the owner is brought back in line",
			existing:           newPDB(2, owner),
			wantMaxUnavailable: 1,
			wantUID:            owner.UID,
		},
		{
			name:               "unowned budget with the same name is left alone",
			existing:           newPDB(2, nil),
			wantErr:            true,
			wantMaxUnavailable: 2,
		},
		{
			name:               "budget controlled by another object is left alone",
			existing:           newPDB(2, other),
			wantErr:            true,
			wantMaxUnavailable: 2,
			wantUID:            other.UID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.existing != nil {
				builder = builder.WithObjects(tt.existing)
			}
			cli := builder.Build()

			_, err := Reconcile(context.Background(), cli, *newPDB(1, nil), owner)

			if tt.wantErr {
				require.ErrorIs(t, err, reconciler.ErrNotControlledByOwner)
			} else {
				require.NoError(t, err)
			}
			got := &policyv1.PodDisruptionBudget{}
			require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, got))
			assert.Equal(t, tt.wantMaxUnavailable, got.Spec.MaxUnavailable.IntVal)
			if tt.wantUID == "" {
				assert.Nil(t, metav1.GetControllerOf(got))
			} else {
				require.NotNil(t, metav1.GetControllerOf(got))
				assert.Equal(t, tt.wantUID, metav1.GetControllerOf(got).UID)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name        string
		existing    *policyv1.PodDisruptionBudget
		wantDeleted bool
	}{
		{name: "missing budget is not an error"},
		{name: "budget controlled by the owner is deleted", existing: newPDB(1, owner), wantDeleted: true},
		{name: "unowned budget is kept", existing: newPDB(1, nil)},
		{name: "budget controlled by another object is kept", existing: newPDB(1, other)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.existing != nil {
				builder = builder.WithObjects(tt.existing)
			}
			cli := builder.Build()

			require.NoError(t, Delete(context.Background(), cli, namespace, name, owner))

			err := cli.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, &policyv1.PodDisruptionBudget{})
			if tt.existing == nil || tt.wantDeleted {
				assert.True(t, apierrors.IsNotFound(err), "expected the budget to be absent, got %v", err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
