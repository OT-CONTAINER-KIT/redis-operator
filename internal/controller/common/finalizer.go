package common

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// AddFinalizer add finalizer for graceful deletion
func AddFinalizer(ctx context.Context, cr client.Object, finalizer string, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, finalizer) {
		controllerutil.AddFinalizer(cr, finalizer)
		return cl.Update(ctx, cr)
	}
	return nil
}
