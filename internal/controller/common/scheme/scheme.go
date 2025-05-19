package scheme

import (
	"sync"

	rvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redis/v1beta2"
	rcvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/rediscluster/v1beta2"
	rrvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redisreplication/v1beta2"
	rsvb2 "github.com/OT-CONTAINER-KIT/redis-operator/api/redissentinel/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	Scheme      = clientgoscheme.Scheme
	oncev1beta2 sync.Once
)

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
		rvb2.AddToScheme,
		rcvb2.AddToScheme,
		rrvb2.AddToScheme,
		rsvb2.AddToScheme,
	}
	mustAddSchemeOnce(&oncev1beta2, schemes)
}
