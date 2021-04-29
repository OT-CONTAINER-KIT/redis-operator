module redis-operator

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	sigs.k8s.io/controller-runtime v0.7.0
)
