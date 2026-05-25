package controllerutil

import (
	"context"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ResourceWatcher implements handler.EventHandler and is used to trigger reconciliation when
// a watched object changes. It's designed to only be used for a single type of object.
// If multiple types should be watched, one ResourceWatcher for each type should be used.
//
// mu guards concurrent Watch calls racing handleEvent reads. Under
// MAX_CONCURRENT_RECONCILES > 1 the old unsynchronised map tripped the
// Go race detector immediately at scale (#1739). Receivers are also
// promoted to pointer so the mutex actually protects a single struct
// rather than whichever copy the caller happens to hold.
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

// Watch will add a new object to watch.
func (w *ResourceWatcher) Watch(ctx context.Context, watchedName, dependentName types.NamespacedName) {
	w.mu.Lock()
	defer w.mu.Unlock()

	existing, hasExisting := w.watched[watchedName]
	if !hasExisting {
		existing = []types.NamespacedName{}
	}

	for _, dependent := range existing {
		if dependent == dependentName {
			return
		}
	}
	w.watched[watchedName] = append(existing, dependentName)
}

func (w *ResourceWatcher) Create(ctx context.Context, event event.CreateEvent, queue workqueue.RateLimitingInterface) {
	w.handleEvent(event.Object, queue)
}

func (w *ResourceWatcher) Update(ctx context.Context, event event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	w.handleEvent(event.ObjectOld, queue)
}

func (w *ResourceWatcher) Delete(ctx context.Context, event event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	w.handleEvent(event.Object, queue)
}

func (w *ResourceWatcher) Generic(ctx context.Context, event event.GenericEvent, queue workqueue.RateLimitingInterface) {
	w.handleEvent(event.Object, queue)
}

// handleEvent is called when an event is received for an object.
// It will check if the object is being watched and trigger a reconciliation for
// the dependent object.
func (w *ResourceWatcher) handleEvent(meta metav1.Object, queue workqueue.RateLimitingInterface) {
	changedObjectName := types.NamespacedName{
		Name:      meta.GetName(),
		Namespace: meta.GetNamespace(),
	}

	// Snapshot the dependent list under the read lock so the queue.Add
	// loop does not hold the lock while the controller is enqueueing.
	w.mu.RLock()
	dependents := w.watched[changedObjectName]
	snapshot := append([]types.NamespacedName(nil), dependents...)
	w.mu.RUnlock()

	// Enqueue reconciliation for each dependent object.
	for _, dep := range snapshot {
		queue.Add(reconcile.Request{
			NamespacedName: dep,
		})
	}
}
