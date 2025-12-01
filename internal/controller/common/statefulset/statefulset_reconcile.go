package statefulset

import (
	"context"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/reconciler"
	"github.com/OT-CONTAINER-KIT/redis-operator/internal/util/maps"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Params struct {
	Name            string
	Namespace       string
	Replicas        int32
	ServiceName     string
	PodTemplateSpec corev1.PodTemplateSpec
}

func New(params Params) appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: params.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: params.ServiceName,
			Replicas:    ptr.To(params.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: params.PodTemplateSpec.Labels,
			},
			Template: params.PodTemplateSpec,
		},
	}
}

func Reconcile(ctx context.Context, k8sClient client.Client, expected appsv1.StatefulSet, owner client.Object) (appsv1.StatefulSet, error) {
	reconciled, err := reconciler.Reconcile(ctx, reconciler.Params{
		Client:   k8sClient,
		Owner:    owner,
		Expected: &expected,
		NeedUpdate: func(existed client.Object) bool {
			return needUpdate(&expected, existed.(*appsv1.StatefulSet))
		},
		Update: func(existed client.Object) {
			update(&expected, existed.(*appsv1.StatefulSet))
		},
	})
	return *reconciled.(*appsv1.StatefulSet), err
}

func needUpdate(expected, existed *appsv1.StatefulSet) bool {
	return !maps.IsSubset(expected.Labels, existed.Labels) ||
		!maps.IsSubset(expected.Annotations, existed.Annotations) ||
		!equality.Semantic.DeepEqual(expected.Spec, existed.Spec)
}

func update(expected, existed *appsv1.StatefulSet) {
	existed.Labels = maps.Merge(existed.Labels, expected.Labels)
	existed.Annotations = maps.Merge(existed.Annotations, expected.Annotations)
	existed.Spec = expected.Spec
}
