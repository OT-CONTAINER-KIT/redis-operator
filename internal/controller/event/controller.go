package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/agent/events"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// eventHandledAnnotationKey is used to mark the event has been handled
	eventHandledAnnotationKey = "redis.opstreelabs.in/event-handled"
)

// EventReconciler reconciles Kubernetes Events for Redis role changes
type EventReconciler struct {
	client.Client
}

// Reconcile handles role change events and updates Pod labels
func (r *EventReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Event
	var event corev1.Event
	if err := r.Get(ctx, req.NamespacedName, &event); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if this event has already been handled
	if r.isEventHandled(&event) {
		logger.V(1).Info("Event already handled, skipping",
			"event", event.Name,
			"count", event.Count)
		return ctrl.Result{}, nil
	}

	logger.Info("Processing role change event",
		"pod", event.InvolvedObject.Name,
		"namespace", event.InvolvedObject.Namespace,
		"count", event.Count)

	// Parse the structured message
	var roleData events.RoleChangeEventData
	if err := json.Unmarshal([]byte(event.Message), &roleData); err != nil {
		logger.Error(err, "Failed to parse role change event message", "message", event.Message)
		return ctrl.Result{}, nil
	}

	// Update Pod label based on role change
	if err := r.updatePodRoleLabel(ctx, roleData); err != nil {
		logger.Error(err, "Failed to update Pod role label",
			"pod", roleData.PodName,
			"namespace", roleData.Namespace,
			"role", roleData.CurrentRole)
		return ctrl.Result{RequeueAfter: time.Second * 30}, err
	}

	// Mark event as handled
	if err := r.markEventAsHandled(ctx, &event); err != nil {
		logger.Error(err, "Failed to mark event as handled")
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	logger.Info("Successfully processed role change event",
		"pod", roleData.PodName,
		"namespace", roleData.Namespace,
		"previousRole", roleData.PreviousRole,
		"currentRole", roleData.CurrentRole)

	return ctrl.Result{}, nil
}

// isEventHandled checks if the event has already been processed
func (r *EventReconciler) isEventHandled(event *corev1.Event) bool {
	count := fmt.Sprintf("%d", event.Count)
	annotations := event.GetAnnotations()
	if annotations != nil && annotations[eventHandledAnnotationKey] == count {
		return true
	}
	return false
}

// markEventAsHandled marks the event as processed to prevent duplicate processing
func (r *EventReconciler) markEventAsHandled(ctx context.Context, event *corev1.Event) error {
	patch := client.MergeFrom(event.DeepCopy())
	if event.Annotations == nil {
		event.Annotations = make(map[string]string)
	}
	event.Annotations[eventHandledAnnotationKey] = fmt.Sprintf("%d", event.Count)
	return r.Client.Patch(ctx, event, patch)
}

// updatePodRoleLabel updates the Pod's role label
func (r *EventReconciler) updatePodRoleLabel(ctx context.Context, roleData events.RoleChangeEventData) error {
	// Fetch the Pod
	pod := &corev1.Pod{}
	podKey := types.NamespacedName{
		Name:      roleData.PodName,
		Namespace: roleData.Namespace,
	}

	if err := r.Get(ctx, podKey, pod); err != nil {
		return fmt.Errorf("failed to get pod %s/%s: %w", roleData.Namespace, roleData.PodName, err)
	}

	// Update the role label
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}

	// Convert Redis role to operator role format
	operatorRole := convertRedisRoleToOperatorRole(roleData.CurrentRole)
	pod.Labels["redis-role"] = operatorRole

	// Update the Pod
	if err := r.Update(ctx, pod); err != nil {
		return fmt.Errorf("failed to update pod %s/%s: %w", roleData.Namespace, roleData.PodName, err)
	}

	return nil
}

// convertRedisRoleToOperatorRole converts Redis role to operator role format
func convertRedisRoleToOperatorRole(redisRole string) string {
	switch redisRole {
	case "master":
		return "master"
	case "slave":
		return "slave"
	default:
		return "unknown"
	}
}

// roleChangeEventPredicate filters events to only process Redis role change events
func roleChangeEventPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isRoleChangeEvent(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isRoleChangeEvent(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false // We don't care about deleted events
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return isRoleChangeEvent(e.Object)
		},
	}
}

// isRoleChangeEvent checks if the event is a Redis role change event
func isRoleChangeEvent(obj client.Object) bool {
	event, ok := obj.(*corev1.Event)
	if !ok {
		return false
	}

	// Only process role change events from redis-role-detector for Pod objects
	return event.Reason == events.RoleChangedReason &&
		event.Source.Component == events.EventSourceComponent &&
		event.InvolvedObject.Kind == events.InvolvedObjectPod
}

// SetupWithManager sets up the controller with the Manager
func (r *EventReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Event{}).
		WithOptions(controller.Options{}).
		WithEventFilter(roleChangeEventPredicate()).
		Complete(r)
}
