package service

import (
	"context"
	"fmt"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/reconciler"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Reconcile(
	ctx context.Context,
	cli client.Client,
	expected corev1.Service,
	owner client.Object,
) (corev1.Service, error) {
	reconciled, err := reconciler.Reconcile(ctx, reconciler.Params{
		Client:   cli,
		Owner:    owner,
		Expected: &expected,
		NeedUpdate: func(existed client.Object) bool {
			return needUpdate(&expected, existed.(*corev1.Service))
		},
		Update: func(existed client.Object) {
			update(&expected, existed.(*corev1.Service))
		},
	})
	// reconciler.Reconcile returns (nil, err) on internal error paths.
	// Avoid panicking via nil-interface type assertion (see GH #1753).
	if err != nil || reconciled == nil {
		return corev1.Service{}, err
	}
	svc, ok := reconciled.(*corev1.Service)
	if !ok || svc == nil {
		return corev1.Service{}, fmt.Errorf("service reconciler returned unexpected type %T", reconciled)
	}
	return *svc, nil
}

func needUpdate(expected, existed *corev1.Service) bool {
	return !maps.IsSubset(expected.Labels, existed.Labels) ||
		!maps.IsSubset(expected.Annotations, existed.Annotations) ||
		!equality.Semantic.DeepEqual(expected.Spec, existed.Spec)
}

func update(expected, existed *corev1.Service) {
	existed.Labels = maps.Merge(existed.Labels, expected.Labels)
	existed.Annotations = maps.Merge(existed.Annotations, expected.Annotations)
	existed.Spec = expected.Spec
}
