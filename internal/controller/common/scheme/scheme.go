package scheme

import (
	"sync"

	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var addToSchemeV1beta2 sync.Once

func mustAddSchemeOnce(once *sync.Once, schemes []func(scheme *runtime.Scheme) error) {
	once.Do(func() {
		for _, s := range schemes {
			if err := s(clientgoscheme.Scheme); err != nil {
				panic(err)
			}
		}
	})
}

func SetupV1beta2Scheme() {
	schemes := []func(scheme *runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		redisv1beta2.AddToScheme,
	}
	mustAddSchemeOnce(&addToSchemeV1beta2, schemes)
}
