package controllerutil

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRequeueEConflict(t *testing.T) {
	ctx := context.TODO()

	t.Run("conflict error is requeued without surfacing the error", func(t *testing.T) {
		conflictErr := apierrors.NewConflict(
			schema.GroupResource{Group: "redis.redis.opstreelabs.in", Resource: "redisclusters"},
			"test",
			errors.New("the object has been modified; please apply your changes to the latest version and try again"),
		)
		result, err := RequeueEConflict(ctx, conflictErr, "update status")
		assert.NoError(t, err, "conflict errors should be swallowed and retried")
		assert.True(t, result.Requeue, "conflict errors should trigger a requeue")
	})

	t.Run("non-conflict error is surfaced to the caller", func(t *testing.T) {
		otherErr := errors.New("boom")
		result, err := RequeueEConflict(ctx, otherErr, "update status")
		assert.ErrorIs(t, err, otherErr, "non-conflict errors should be returned unchanged")
		assert.False(t, result.Requeue)
	})
}
