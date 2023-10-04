package k8sutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestGenerateServiceDef(t *testing.T) {
	tests := []struct {
		name          string
		serviceMeta   metav1.ObjectMeta
		enableMetrics bool
		headless      bool
		serviceType   string
		expected      *corev1.Service
	}{
		{
			name: "Test sentinel with ClusterIP service type",
			serviceMeta: metav1.ObjectMeta{
				Name: "test-service",
				Labels: map[string]string{
					"role": "sentinel",
				},
			},
			enableMetrics: false,
			headless:      false,
			serviceType:   "ClusterIP",
			expected: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-service",
					Labels: map[string]string{
						"role": "sentinel",
					},
					OwnerReferences: []metav1.OwnerReference{
						{},
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "sentinel-client",
							Port:       sentinelPort,
							TargetPort: intstr.FromInt(int(sentinelPort)),
							Protocol:   corev1.ProtocolTCP,
						},
					},
					Selector:  map[string]string{"role": "sentinel"},
					ClusterIP: "",
					Type:      corev1.ServiceTypeClusterIP,
				},
			},
		},
		{
			name: "Test sentinel with headless service",
			serviceMeta: metav1.ObjectMeta{
				Name: "test-service",
				Labels: map[string]string{
					"role": "sentinel",
				},
			},
			enableMetrics: false,
			headless:      true,
			serviceType:   "ClusterIP",
			expected: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-service",
					Labels: map[string]string{
						"role": "sentinel",
					},
					OwnerReferences: []metav1.OwnerReference{
						{},
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "sentinel-client",
							Port:       sentinelPort,
							TargetPort: intstr.FromInt(int(sentinelPort)),
							Protocol:   corev1.ProtocolTCP,
						},
					},
					Selector:  map[string]string{"role": "sentinel"},
					ClusterIP: "None",
					Type:      corev1.ServiceTypeClusterIP,
				},
			},
		},
		{
			name: "Test redis with ClusterIP service type",
			serviceMeta: metav1.ObjectMeta{
				Name: "test-redis-service",
				Labels: map[string]string{
					"role": "redis",
				},
			},
			enableMetrics: false,
			headless:      false,
			serviceType:   "ClusterIP",
			expected: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-redis-service",
					Labels: map[string]string{
						"role": "redis",
					},
					OwnerReferences: []metav1.OwnerReference{
						{},
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "redis-client",
							Port:       redisPort,
							TargetPort: intstr.FromInt(int(redisPort)),
							Protocol:   corev1.ProtocolTCP,
						},
					},
					Selector:  map[string]string{"role": "redis"},
					ClusterIP: "",
					Type:      corev1.ServiceTypeClusterIP,
				},
			},
		},
		{
			name: "Test redis with headless service",
			serviceMeta: metav1.ObjectMeta{
				Name: "test-redis-headless-service",
				Labels: map[string]string{
					"role": "redis",
				},
			},
			enableMetrics: false,
			headless:      true,
			serviceType:   "ClusterIP",
			expected: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-redis-headless-service",
					Labels: map[string]string{
						"role": "redis",
					},
					OwnerReferences: []metav1.OwnerReference{
						{},
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "redis-client",
							Port:       redisPort,
							TargetPort: intstr.FromInt(int(redisPort)),
							Protocol:   corev1.ProtocolTCP,
						},
					},
					Selector:  map[string]string{"role": "redis"},
					ClusterIP: "None",
					Type:      corev1.ServiceTypeClusterIP,
				},
			},
		},
		{
			name: "Test redis with ClusterIP service type and metrics enabled",
			serviceMeta: metav1.ObjectMeta{
				Name: "test-redis-metrics-service",
				Labels: map[string]string{
					"role": "redis",
				},
			},
			enableMetrics: true,
			headless:      false,
			serviceType:   "ClusterIP",
			expected: &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-redis-metrics-service",
					Labels: map[string]string{
						"role": "redis",
					},
					OwnerReferences: []metav1.OwnerReference{
						{},
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "redis-client",
							Port:       redisPort,
							TargetPort: intstr.FromInt(int(redisPort)),
							Protocol:   corev1.ProtocolTCP,
						},
						*enableMetricsPort(),
					},
					Selector:  map[string]string{"role": "redis"},
					ClusterIP: "",
					Type:      corev1.ServiceTypeClusterIP,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := generateServiceDef(tt.serviceMeta, tt.enableMetrics, metav1.OwnerReference{}, tt.headless, tt.serviceType)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
