package configmap

import (
	"context"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/reconciler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestNeedUpdate(t *testing.T) {
	expected := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "sentinel"}},
		Data:       map[string]string{"redis-sentinel-additional.conf": "tls-replication yes\n"},
	}

	tests := []struct {
		name    string
		existed *corev1.ConfigMap
		want    bool
	}{
		{
			name: "identical",
			existed: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "sentinel"}},
				Data:       map[string]string{"redis-sentinel-additional.conf": "tls-replication yes\n"},
			},
			want: false,
		},
		{
			name: "extra labels set by someone else are kept",
			existed: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "sentinel", "owner": "team"}},
				Data:       map[string]string{"redis-sentinel-additional.conf": "tls-replication yes\n"},
			},
			want: false,
		},
		{
			name: "edited data is restored",
			existed: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "sentinel"}},
				Data:       map[string]string{"redis-sentinel-additional.conf": "tls-replication no\n"},
			},
			want: true,
		},
		{
			name: "missing label",
			existed: &corev1.ConfigMap{
				Data: map[string]string{"redis-sentinel-additional.conf": "tls-replication yes\n"},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, needUpdate(expected, tt.existed))
		})
	}
}

func TestReconcileOwnership(t *testing.T) {
	const (
		namespace = "default"
		name      = "redis-replication-s-tls"
	)
	// Any namespaced kind registered in the client-go scheme can stand in for
	// the RedisReplication: only its UID decides control.
	owner := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "redis-replication", Namespace: namespace, UID: "owner-uid"}}
	other := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "redis-replication", Namespace: namespace, UID: "other-uid"}}
	expectedData := map[string]string{"redis-sentinel-additional.conf": "tls-replication yes\n"}
	foreignData := map[string]string{"app.conf": "unrelated"}

	existing := func(data map[string]string, controller *corev1.Secret) *corev1.ConfigMap {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Data:       data,
		}
		if controller != nil {
			require.NoError(t, controllerutil.SetControllerReference(controller, cm, scheme.Scheme))
		}
		return cm
	}

	tests := []struct {
		name     string
		existing *corev1.ConfigMap
		wantErr  bool
		wantData map[string]string
		wantUID  types.UID
	}{
		{
			name:     "missing ConfigMap is created and controlled by the owner",
			wantData: expectedData,
			wantUID:  owner.UID,
		},
		{
			name:     "ConfigMap controlled by the owner is brought back in line",
			existing: existing(map[string]string{"redis-sentinel-additional.conf": "edited"}, owner),
			wantData: expectedData,
			wantUID:  owner.UID,
		},
		{
			name:     "unowned ConfigMap with the same name is left alone",
			existing: existing(foreignData, nil),
			wantErr:  true,
			wantData: foreignData,
		},
		{
			name:     "ConfigMap controlled by another object with the same name is left alone",
			existing: existing(foreignData, other),
			wantErr:  true,
			wantData: foreignData,
			wantUID:  other.UID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.existing != nil {
				builder = builder.WithObjects(tt.existing)
			}
			cli := builder.Build()
			expected := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{"app": "sentinel"}},
				Data:       expectedData,
			}

			_, err := Reconcile(context.Background(), cli, expected, owner)

			if tt.wantErr {
				require.ErrorIs(t, err, reconciler.ErrNotControlledByOwner)
			} else {
				require.NoError(t, err)
			}
			got := &corev1.ConfigMap{}
			require.NoError(t, cli.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, got))
			assert.Equal(t, tt.wantData, got.Data)
			controller := metav1.GetControllerOf(got)
			if tt.wantUID == "" {
				assert.Nil(t, controller)
				return
			}
			require.NotNil(t, controller)
			assert.Equal(t, tt.wantUID, controller.UID)
		})
	}
}

func TestUpdate(t *testing.T) {
	expected := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "sentinel"}},
		Data:       map[string]string{"redis-sentinel-additional.conf": "tls-replication yes\n"},
	}
	existed := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"owner": "team"}},
		Data:       map[string]string{"redis-sentinel-additional.conf": "tls-replication no\n", "stale": "x"},
	}

	update(expected, existed)

	assert.Equal(t, map[string]string{"app": "sentinel", "owner": "team"}, existed.Labels)
	assert.Equal(t, expected.Data, existed.Data)
	assert.False(t, needUpdate(expected, existed))
}
