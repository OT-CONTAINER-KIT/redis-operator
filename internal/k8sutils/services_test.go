package k8sutils

import (
	"context"
	"fmt"
	"testing"

	"github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8sClientFake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var defaultExporterPortProvider exporterPortProvider = func() (int, bool) {
	return common.RedisExporterPort, true
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
			port:          common.SentinelPort,
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
							Port:       common.SentinelPort,
							TargetPort: intstr.FromInt(int(common.SentinelPort)),
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
			port:          common.SentinelPort,
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
							Port:       common.SentinelPort,
							TargetPort: intstr.FromInt(int(common.SentinelPort)),
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
			port:          common.RedisPort,
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
							Port:       common.RedisPort,
							TargetPort: intstr.FromInt(int(common.RedisPort)),
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
			port:          common.RedisPort,
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
							Port:       common.RedisPort,
							TargetPort: intstr.FromInt(int(common.RedisPort)),
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
			port:          common.RedisPort,
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
							Port:       common.RedisPort,
							TargetPort: intstr.FromInt(int(common.RedisPort)),
							Protocol:   corev1.ProtocolTCP,
						},
						*enableMetricsPort(common.RedisExporterPort),
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

func Test_createService(t *testing.T) {
	tests := []struct {
		name    string
		service *corev1.Service
		exist   bool
		wantErr bool
	}{
		{
			name: "Service created successfully",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-namespace",
				},
			},
			wantErr: false,
		},
		{
			name: "Service creation failed already exists",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-namespace",
				},
			},
			exist:   true,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var k8sClient *k8sClientFake.Clientset
			if tt.exist {
				k8sClient = k8sClientFake.NewSimpleClientset(tt.service.DeepCopyObject())
			} else {
				k8sClient = k8sClientFake.NewSimpleClientset()
			}

			err := createService(context.TODO(), k8sClient, tt.service.GetNamespace(), tt.service)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify the service was created
				got, err := k8sClient.CoreV1().Services(tt.service.GetNamespace()).Get(context.TODO(), tt.service.GetName(), metav1.GetOptions{})
				assert.NoError(t, err)
				assert.Equal(t, tt.service, got)
			}
		})
	}
}

func Test_updateService(t *testing.T) {
	tests := []struct {
		name              string
		serviceName       string
		servinceNamespace string
		current           *corev1.Service
		updated           *corev1.Service
		wantErr           bool
	}{
		{
			name:              "Service updated successfully",
			serviceName:       "test-service",
			servinceNamespace: "test-namespace",
			current: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-namespace",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "test-port",
							Port:       6379,
							TargetPort: intstr.FromInt(6379),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			},
			updated: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-namespace",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "fake-port",
							Port:       6380,
							TargetPort: intstr.FromInt(6380),
							Protocol:   corev1.ProtocolUDP,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:              "Service does not exist",
			serviceName:       "test-service",
			servinceNamespace: "test-namespace",
			current:           &corev1.Service{},
			updated: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service",
					Namespace: "test-namespace",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "fake-port",
							Port:       6380,
							TargetPort: intstr.FromInt(6380),
							Protocol:   corev1.ProtocolUDP,
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k8sClient := k8sClientFake.NewSimpleClientset(tt.current.DeepCopyObject())

			err := updateService(context.TODO(), k8sClient, tt.servinceNamespace, tt.updated)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify the service was updated
				got, err := k8sClient.CoreV1().Services(tt.servinceNamespace).Get(context.TODO(), tt.serviceName, metav1.GetOptions{})
				assert.NoError(t, err)
				assert.Equal(t, tt.updated, got)
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
			var k8sClient *k8sClientFake.Clientset
			if tt.have != nil {
				k8sClient = k8sClientFake.NewSimpleClientset(tt.have.DeepCopyObject())
			} else {
				k8sClient = k8sClientFake.NewSimpleClientset()
			}

			got, err := getService(context.TODO(), k8sClient, tt.want.GetNamespace(), tt.want.GetName())
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestReconcileMetricsService(t *testing.T) {
	namespace := "test-namespace"
	serviceName := "test-redis-metrics"
	selectorLabels := map[string]string{"app": "test-redis"}
	ownerDef := metav1.OwnerReference{
		APIVersion: "redis.redis.opstreelabs.in/v1beta2",
		Kind:       "RedisReplication",
		Name:       "test-redis",
		UID:        "abc-123",
	}
	existingMetricsService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
	}

	tests := []struct {
		name           string
		enabled        bool
		existing       *corev1.Service
		injectGetError bool
		wantErr        bool
		wantPresent    bool
		wantVerbs      []string
	}{
		{
			name:        "enabled creates metrics Service when absent",
			enabled:     true,
			wantPresent: true,
			wantVerbs:   []string{"get", "create"},
		},
		{
			name:        "enabled updates metrics Service when present",
			enabled:     true,
			existing:    existingMetricsService,
			wantPresent: true,
			wantVerbs:   []string{"get", "update"},
		},
		{
			name:        "disabled deletes orphaned metrics Service",
			enabled:     false,
			existing:    existingMetricsService,
			wantPresent: false,
			wantVerbs:   []string{"get", "delete"},
		},
		{
			name:        "disabled is a no-op when metrics Service is absent",
			enabled:     false,
			wantPresent: false,
			wantVerbs:   []string{"get"},
		},
		{
			name:           "disabled propagates non-NotFound get errors",
			enabled:        false,
			injectGetError: true,
			wantErr:        true,
			wantPresent:    false,
			wantVerbs:      []string{"get"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []runtime.Object
			if tt.existing != nil {
				objs = append(objs, tt.existing.DeepCopyObject())
			}
			client := k8sClientFake.NewSimpleClientset(objs...)
			if tt.injectGetError {
				client.PrependReactor("get", "services", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewInternalError(fmt.Errorf("simulated apiserver failure"))
				})
			}

			err := ReconcileMetricsService(context.TODO(), namespace, serviceName,
				selectorLabels, ownerDef, common.RedisExporterPort, tt.enabled, client)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			var gotVerbs []string
			for _, action := range client.Actions() {
				if action.GetResource().Resource == "services" {
					gotVerbs = append(gotVerbs, action.GetVerb())
				}
			}
			assert.Equal(t, tt.wantVerbs, gotVerbs)

			got, getErr := client.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
			if tt.wantPresent {
				assert.NoError(t, getErr)
				assert.Equal(t, "metrics", got.Labels["app.kubernetes.io/component"])
				assert.Len(t, got.Spec.Ports, 1)
				assert.Equal(t, int32(common.RedisExporterPort), got.Spec.Ports[0].Port)
			} else if !tt.injectGetError {
				assert.True(t, apierrors.IsNotFound(getErr))
			}
		})
	}
}
