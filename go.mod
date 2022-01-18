module redis-operator

go 1.16

require (
	github.com/banzaicloud/k8s-objectmatcher v1.7.0
	github.com/go-logr/logr v1.2.2
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	sigs.k8s.io/controller-runtime v0.11.0
)
