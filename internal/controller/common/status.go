package common

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func UpdateStatus(ctx context.Context, client client.Client, obj client.Object) error {
	return client.Status().Update(ctx, obj)
}
