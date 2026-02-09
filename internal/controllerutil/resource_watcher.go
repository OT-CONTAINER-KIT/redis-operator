package controllerutil

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ResourceWatcher triggers reconciliation of dependent objects when a watched object changes.
type ResourceWatcher struct {
	mu      sync.RWMutex
	watched map[types.NamespacedName][]types.NamespacedName
}

var _ handler.EventHandler = &ResourceWatcher{}

// NewResourceWatcher will create a new ResourceWatcher with no watched objects.
func NewResourceWatcher() *ResourceWatcher {
	return &ResourceWatcher{
		watched: make(map[types.NamespacedName][]types.NamespacedName),
	}
}

// Watch adds a dependent object to be reconciled when watchedName changes.
func (w *ResourceWatcher) Watch(ctx context.Context, watchedName, dependentName types.NamespacedName) {
	w.mu.Lock()
	defer w.mu.Unlock()

	existing := w.watched[watchedName]
	for _, dep := range existing {
		if dep == dependentName {
			return
		}
	}
	w.watched[watchedName] = append(existing, dependentName)
}

func (w *ResourceWatcher) Create(ctx context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	w.handleEvent(e.Object, q)
}

func (w *ResourceWatcher) Update(ctx context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	// name/namespace can’t change, so old vs new usually doesn’t matter; new is conventional
	w.handleEvent(e.ObjectNew, q)
}

func (w *ResourceWatcher) Delete(ctx context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	w.handleEvent(e.Object, q)
}

func (w *ResourceWatcher) Generic(ctx context.Context, e event.TypedGenericEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	w.handleEvent(e.Object, q)
}

// handleEvent is called when an event is received for an object.
// It will check if the object is being watched and trigger a reconciliation for
// the dependent object.
func (w *ResourceWatcher) handleEvent(obj client.Object, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}

	w.mu.RLock()
	deps := append([]types.NamespacedName(nil), w.watched[key]...) // copy to avoid holding lock while enqueueing
	w.mu.RUnlock()

	for _, dep := range deps {
		q.Add(reconcile.Request{NamespacedName: dep})
	}
}
