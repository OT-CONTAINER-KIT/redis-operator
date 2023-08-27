package k8sutils

import (
	"context"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	redisPort             = 6379
	sentinelPort          = 26379
	redisExporterPort     = 9121
	redisExporterPortName = "redis-exporter"
)

var (
	serviceType corev1.ServiceType
)

// generateServiceDef generates service definition for Redis
func generateServiceDef(serviceMeta metav1.ObjectMeta, enableMetrics bool, ownerDef metav1.OwnerReference, headless bool, serviceType string) *corev1.Service {
	var PortName string
	var PortNum int32
	if serviceMeta.Labels["role"] == "sentinel" {
		PortName = "sentinel-client"
		PortNum = sentinelPort
	} else {
		PortName = "redis-client"
		PortNum = redisPort
	}
	service := &corev1.Service{
		TypeMeta:   generateMetaInformation("Service", "v1"),
		ObjectMeta: serviceMeta,
		Spec: corev1.ServiceSpec{
			Type:      generateServiceType(serviceType),
			ClusterIP: "",
			Selector:  serviceMeta.GetLabels(),
			Ports: []corev1.ServicePort{
				{
					Name:       PortName,
					Port:       PortNum,
					TargetPort: intstr.FromInt(int(PortNum)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	if headless {
		service.Spec.ClusterIP = "None"
	}
	if enableMetrics {
		redisExporterService := enableMetricsPort()
		service.Spec.Ports = append(service.Spec.Ports, *redisExporterService)
	}
	AddOwnerRefToObject(service, ownerDef)
	return service
}

// enableMetricsPort will enable the metrics for Redis service
func enableMetricsPort() *corev1.ServicePort {
	return &corev1.ServicePort{
		Name:       redisExporterPortName,
		Port:       redisExporterPort,
		TargetPort: intstr.FromInt(int(redisExporterPort)),
		Protocol:   corev1.ProtocolTCP,
	}
}

// generateServiceType generates service type
func generateServiceType(k8sServiceType string) corev1.ServiceType {
	switch k8sServiceType {
	case "LoadBalancer":
		serviceType = corev1.ServiceTypeLoadBalancer
	case "NodePort":
		serviceType = corev1.ServiceTypeNodePort
	case "ClusterIP":
		serviceType = corev1.ServiceTypeClusterIP
	default:
		serviceType = corev1.ServiceTypeClusterIP
	}
	return serviceType
}

// createService is a method to create service is Kubernetes
func createService(namespace string, service *corev1.Service) error {
	logger := serviceLogger(namespace, service.Name)
	_, err := generateK8sClient().CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Redis service creation is failed")
		return err
	}
	logger.V(1).Info("Redis service creation is successful")
	return nil
}

// updateService is a method to update service is Kubernetes
func updateService(namespace string, service *corev1.Service) error {
	logger := serviceLogger(namespace, service.Name)
	_, err := generateK8sClient().CoreV1().Services(namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Redis service update failed")
		return err
	}
	logger.V(1).Info("Redis service updated successfully")
	return nil
}

// getService is a method to get service is Kubernetes
func getService(namespace string, service string) (*corev1.Service, error) {
	logger := serviceLogger(namespace, service)
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("Service", "v1"),
	}
	serviceInfo, err := generateK8sClient().CoreV1().Services(namespace).Get(context.TODO(), service, getOpts)
	if err != nil {
		logger.Info("Redis service get action is failed")
		return nil, err
	}
	logger.V(1).Info("Redis service get action is successful")
	return serviceInfo, nil
}

func serviceLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Service.Namespace", namespace, "Request.Service.Name", name)
	return reqLogger
}

// CreateOrUpdateService method will create or update Redis service
func CreateOrUpdateService(namespace string, serviceMeta metav1.ObjectMeta, ownerDef metav1.OwnerReference, enableMetrics, headless bool, serviceType string) error {
	logger := serviceLogger(namespace, serviceMeta.Name)
	serviceDef := generateServiceDef(serviceMeta, enableMetrics, ownerDef, headless, serviceType)
	storedService, err := getService(namespace, serviceMeta.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(serviceDef); err != nil {
				logger.Error(err, "Unable to patch redis service with compare annotations")
			}
			return createService(namespace, serviceDef)
		}
		return err
	}
	return patchService(storedService, serviceDef, namespace)
}

// patchService will patch Redis Kubernetes service
func patchService(storedService *corev1.Service, newService *corev1.Service, namespace string) error {
	logger := serviceLogger(namespace, storedService.Name)
	// We want to try and keep this atomic as possible.
	newService.ResourceVersion = storedService.ResourceVersion
	newService.CreationTimestamp = storedService.CreationTimestamp
	newService.ManagedFields = storedService.ManagedFields

	if newService.Spec.Type == generateServiceType("ClusterIP") {
		newService.Spec.ClusterIP = storedService.Spec.ClusterIP
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(storedService, newService,
		patch.IgnoreStatusFields(),
		patch.IgnoreField("kind"),
		patch.IgnoreField("apiVersion"),
	)
	if err != nil {
		logger.Error(err, "Unable to patch redis service with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		logger.V(1).Info("Changes in service Detected, Updating...", "patch", string(patchResult.Patch))

		for key, value := range storedService.Annotations {
			if _, present := newService.Annotations[key]; !present {
				newService.Annotations[key] = value
			}
		}
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newService); err != nil {
			logger.Error(err, "Unable to patch redis service with comparison object")
			return err
		}
		logger.V(1).Info("Syncing Redis service with defined properties")
		return updateService(namespace, newService)
	}
	logger.V(1).Info("Redis service is already in-sync")
	return nil
}
