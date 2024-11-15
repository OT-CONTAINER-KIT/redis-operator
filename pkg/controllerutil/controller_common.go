package controllerutil

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func Reconciled() (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func RequeueAfter(ctx context.Context, duration time.Duration, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	keysAndValues = append(keysAndValues, "duration", duration.String())
	if msg == "" {
		msg = "requeue-after"
	}
	log.FromContext(ctx).Info(msg, keysAndValues...)
	return reconcile.Result{
		Requeue:      true,
		RequeueAfter: duration,
	}, nil
}

func RequeueWithError(ctx context.Context, err error, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if msg == "" {
		msg = "requeue with error"
	}
	log.FromContext(ctx).Error(err, msg, keysAndValues...)
	return reconcile.Result{}, err
}

func RequeueWithErrorChecking(ctx context.Context, err error, msg string, keysAndValues ...interface{}) (reconcile.Result, error) {
	if apierrors.IsNotFound(err) {
		return Reconciled()
	}
	return RequeueWithError(ctx, err, msg, keysAndValues...)
}
