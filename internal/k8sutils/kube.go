package k8sutils

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsDeleted(obj client.Object) bool {
	return obj.GetDeletionTimestamp() != nil
}
