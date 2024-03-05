package k8sutils

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
)

var defaultExporterPortProvider exporterPortProvider = func() (int, bool) {
	return redisExporterPort, true
}

func TestGenerateServiceDef(t *testing.T) {
	tests := []struct {
		name          string
		serviceMeta   metav1.ObjectMeta
		enableMetrics exporterPortProvider
		headless      bool
		serviceType   string
		port          int
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
			enableMetrics: disableMetrics,
			headless:      false,
			serviceType:   "ClusterIP",
			port:          sentinelPort,
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
			enableMetrics: disableMetrics,
			headless:      true,
			serviceType:   "ClusterIP",
			port:          sentinelPort,
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
			enableMetrics: disableMetrics,
			headless:      false,
			serviceType:   "ClusterIP",
			port:          redisPort,
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
			enableMetrics: disableMetrics,
			headless:      true,
			serviceType:   "ClusterIP",
			port:          redisPort,
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
			enableMetrics: defaultExporterPortProvider,
			headless:      false,
			serviceType:   "ClusterIP",
			port:          redisPort,
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
						*enableMetricsPort(redisExporterPort),
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
			actual := generateServiceDef(tt.serviceMeta, tt.enableMetrics, metav1.OwnerReference{}, tt.headless, tt.serviceType, tt.port)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestGenerateServiceType(t *testing.T) {
	tests := []struct {
		name         string
		serviceType  string
		expectedType corev1.ServiceType
	}{
		{
			name:         "LoadBalancer service type",
			serviceType:  "LoadBalancer",
			expectedType: corev1.ServiceTypeLoadBalancer,
		},
		{
			name:         "NodePort service type",
			serviceType:  "NodePort",
			expectedType: corev1.ServiceTypeNodePort,
		},
		{
			name:         "ClusterIP service type",
			serviceType:  "ClusterIP",
			expectedType: corev1.ServiceTypeClusterIP,
		},
		{
			name:         "Default service type",
			serviceType:  "InvalidServiceType",
			expectedType: corev1.ServiceTypeClusterIP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualType := generateServiceType(tt.serviceType)
			if actualType != tt.expectedType {
				t.Errorf("Expected service type %v, but got %v", tt.expectedType, actualType)
			}
		})
	}
}

func Test_getService(t *testing.T) {
	tests := []struct {
		name    string
		have    *corev1.Service
		want    *corev1.Service
		wantErr bool
	}{
		{
			name: "Service exists",
			have: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-namespace",
				},
			},
			want: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-namespace",
				},
			},
		},
		{
			name: "Service does not exist",
			have: &corev1.Service{},
			want: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-namespace",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := testr.New(t)
			var k8sClient *k8sClientFake.Clientset
			if tt.have != nil {
				k8sClient = k8sClientFake.NewSimpleClientset(tt.have.DeepCopyObject())
			} else {
				k8sClient = k8sClientFake.NewSimpleClientset()
			}

			got, err := getService(k8sClient, logger, tt.want.GetNamespace(), tt.want.GetName())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
