package service

import (
	"context"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/reconciler"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/maps"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Reconcile(
	ctx context.Context,
	cli client.Client,
	expected corev1.Service,
	owner client.Object,
) (corev1.Service, error) {
	if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(&expected); err != nil {
		return expected, err
	}
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
	return *reconciled.(*corev1.Service), err
}

func needUpdate(expected, existed *corev1.Service) bool {
	return !maps.IsSubset(expected.Labels, existed.Labels) ||
		!maps.IsSubset(expected.Annotations, existed.Annotations) ||
		changed(expected, existed)
}

// changed ignores fields the API server defaults, which DeepEqual reported as a change on every reconcile.
func changed(expected, existed *corev1.Service) bool {
	result, err := patch.DefaultPatchMaker.Calculate(existed, expected, patch.IgnoreStatusFields())
	return err != nil || !result.IsEmpty()
}

func update(expected, existed *corev1.Service) {
	existed.Labels = maps.Merge(existed.Labels, expected.Labels)
	existed.Annotations = maps.Merge(existed.Annotations, expected.Annotations)
	existed.Spec = expected.Spec
}
