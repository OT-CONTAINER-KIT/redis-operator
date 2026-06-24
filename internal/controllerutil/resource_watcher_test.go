package controllerutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func drain(queue workqueue.RateLimitingInterface) []reconcile.Request {
	var got []reconcile.Request
	for queue.Len() > 0 {
		item, _ := queue.Get()
		got = append(got, item.(reconcile.Request))
		queue.Done(item)
	}
	return got
}

func configMapUpdate(namespace, name string) event.UpdateEvent {
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	return event.UpdateEvent{ObjectOld: obj, ObjectNew: obj}
}

func TestResourceWatcherEnqueuesDependentOnWatchedChange(t *testing.T) {
	w := NewResourceWatcher()
	dependent := types.NamespacedName{Namespace: "ns", Name: "my-redis"}
	w.Watch(context.TODO(), types.NamespacedName{Namespace: "ns", Name: "external-config"}, dependent)

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	w.Update(context.TODO(), configMapUpdate("ns", "external-config"), queue)

	assert.Equal(t, []reconcile.Request{{NamespacedName: dependent}}, drain(queue))
}

func TestResourceWatcherIgnoresUnwatchedChange(t *testing.T) {
	w := NewResourceWatcher()
	w.Watch(context.TODO(), types.NamespacedName{Namespace: "ns", Name: "external-config"}, types.NamespacedName{Namespace: "ns", Name: "my-redis"})

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	w.Update(context.TODO(), configMapUpdate("ns", "some-other-configmap"), queue)

	assert.Empty(t, drain(queue))
}

func TestResourceWatcherWatchMany(t *testing.T) {
	w := NewResourceWatcher()
	dependent := types.NamespacedName{Namespace: "ns", Name: "my-redis"}
	// Empty names must be ignored; the rest register under the dependent's namespace.
	w.WatchMany(context.TODO(), dependent, "tls-secret", "", "password-secret")

	for _, secret := range []string{"tls-secret", "password-secret"} {
		queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		w.Update(context.TODO(), configMapUpdate("ns", secret), queue)
		assert.Equal(t, []reconcile.Request{{NamespacedName: dependent}}, drain(queue), "expected enqueue for %s", secret)
	}

	// The skipped empty name must not have registered a watch for "".
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	w.Update(context.TODO(), configMapUpdate("ns", ""), queue)
	assert.Empty(t, drain(queue))
}
