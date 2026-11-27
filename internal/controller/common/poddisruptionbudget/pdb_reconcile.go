package poddisruptionbudget

import (
	"context"
	"fmt"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/reconciler"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/maps"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Reconcile creates the expected PodDisruptionBudget, owned by owner, or brings
// the existing one's labels, annotations and spec in line with it. A budget of
// the same name that owner does not control, such as one a user created by
// hand before the operator could manage it, is left alone: Reconcile returns an
// error wrapping reconciler.ErrNotControlledByOwner. Adopting it would leave
// two controllers fighting over the same object.
func Reconcile(
	ctx context.Context,
	cli client.Client,
	expected policyv1.PodDisruptionBudget,
	owner client.Object,
) (policyv1.PodDisruptionBudget, error) {
	reconciled, err := reconciler.Reconcile(ctx, reconciler.Params{
		Client:                   cli,
		Owner:                    owner,
		Expected:                 &expected,
		RequireControlledByOwner: true,
		NeedUpdate: func(existed client.Object) bool {
			return needUpdate(&expected, existed.(*policyv1.PodDisruptionBudget))
		},
		Update: func(existed client.Object) {
			update(&expected, existed.(*policyv1.PodDisruptionBudget))
		},
	})
	if err != nil || reconciled == nil {
		return expected, err
	}
	return *reconciled.(*policyv1.PodDisruptionBudget), nil
}

// Delete removes the named PodDisruptionBudget when owner controls it. A
// missing budget, or one owner does not control, is not an error.
func Delete(ctx context.Context, cli client.Client, namespace, name string, owner client.Object) error {
	existed := &policyv1.PodDisruptionBudget{}
	err := cli.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, existed)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get PodDisruptionBudget %s/%s: %w", namespace, name, err)
	}
	if !metav1.IsControlledBy(existed, owner) {
		return nil
	}
	log.FromContext(ctx).Info("Deleting resource", "kind", "PodDisruptionBudget", "namespace", namespace, "name", name)
	if err := cli.Delete(ctx, existed); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete PodDisruptionBudget %s/%s: %w", namespace, name, err)
	}
	return nil
}

func needUpdate(expected, existed *policyv1.PodDisruptionBudget) bool {
	return !maps.IsSubset(expected.Labels, existed.Labels) ||
		!maps.IsSubset(expected.Annotations, existed.Annotations) ||
		!equality.Semantic.DeepEqual(expected.Spec.Selector, existed.Spec.Selector) ||
		!equality.Semantic.DeepEqual(expected.Spec.MinAvailable, existed.Spec.MinAvailable) ||
		!equality.Semantic.DeepEqual(expected.Spec.MaxUnavailable, existed.Spec.MaxUnavailable)
}

func update(expected, existed *policyv1.PodDisruptionBudget) {
	existed.Labels = maps.Merge(existed.Labels, expected.Labels)
	existed.Annotations = maps.Merge(existed.Annotations, expected.Annotations)
	existed.Spec.Selector = expected.Spec.Selector
	existed.Spec.MinAvailable = expected.Spec.MinAvailable
	existed.Spec.MaxUnavailable = expected.Spec.MaxUnavailable
}
