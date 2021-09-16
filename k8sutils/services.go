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
	redisPort         = 6379
	redisExporterPort = 9121
)

var (
	serviceType corev1.ServiceType
)

// generateHeadlessServiceDef generates service definition for headless service
func generateHeadlessServiceDef(serviceMeta metav1.ObjectMeta, labels map[string]string, ownerDef metav1.OwnerReference) *corev1.Service {
	service := &corev1.Service{
		TypeMeta:   generateMetaInformation("Service", "core/v1"),
		ObjectMeta: serviceMeta,
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "redis-client",
					Port:       redisPort,
					TargetPort: intstr.FromInt(int(redisPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	AddOwnerRefToObject(service, ownerDef)
	return service
}

// generateServiceDef generates service definition for Redis
func generateServiceDef(serviceMeta metav1.ObjectMeta, labels map[string]string, enableMetrics bool, ownerDef metav1.OwnerReference) *corev1.Service {
	service := &corev1.Service{
		TypeMeta:   generateMetaInformation("Service", "core/v1"),
		ObjectMeta: serviceMeta,
		Spec: corev1.ServiceSpec{
			Type:     generateServiceType("ClusterIP"),
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "redis-client",
					Port:       redisPort,
					TargetPort: intstr.FromInt(int(redisPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
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
		Name:       "redis-exporter",
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
	logger.Info("Redis service creation is successful")
	return nil
}

// updateService is a method to update service is Kubernetes
func updateService(namespace string, service *corev1.Service) error {
	logger := serviceLogger(namespace, service.Name)
	_, err := generateK8sClient().CoreV1().Services(namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Redis service updation is failed")
		return err
	}
	logger.Info("Redis service updation is successful")
	return nil
}

// getService is a method to get service is Kubernetes
func getService(namespace string, service string) (*corev1.Service, error) {
	logger := serviceLogger(namespace, service)
	serviceInfo, err := generateK8sClient().CoreV1().Services(namespace).Get(context.TODO(), service, metav1.GetOptions{})
	if err != nil {
		logger.Info("Redis service get action is failed")
		return nil, err
	}
	logger.Info("Redis service get action is successful")
	return serviceInfo, nil
}

func serviceLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Service.Namespace", namespace, "Request.Service.Name", name)
	return reqLogger
}

// CreateOrUpdateHeadlessService method will create or update Redis headless service
func CreateOrUpdateHeadlessService(namespace string, serviceMeta metav1.ObjectMeta, labels map[string]string, ownerDef metav1.OwnerReference) error {
	logger := serviceLogger(namespace, serviceMeta.Name)
	storedService, err := getService(namespace, serviceMeta.Name)
	serviceDef := generateHeadlessServiceDef(serviceMeta, labels, ownerDef)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(serviceDef); err != nil {
				logger.Error(err, "Unable to patch redis service with comparison object")
				return err
			}
			return createService(namespace, serviceDef)
		}
		return err
	}
	return patchService(storedService, serviceDef, namespace)
}

// CreateOrUpdateService method will create or update Redis service
func CreateOrUpdateService(namespace string, serviceMeta metav1.ObjectMeta, labels map[string]string, ownerDef metav1.OwnerReference, enableMetrics bool) error {
	logger := serviceLogger(namespace, serviceMeta.Name)
	serviceDef := generateServiceDef(serviceMeta, labels, enableMetrics, ownerDef)
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
	patchResult, err := patch.DefaultPatchMaker.Calculate(storedService, newService, patch.IgnoreStatusFields())
	if err != nil {
		logger.Error(err, "Unable to patch redis service with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		newService.Spec.ClusterIP = storedService.Spec.ClusterIP
		newService.ResourceVersion = storedService.ResourceVersion
		newService.CreationTimestamp = storedService.CreationTimestamp
		newService.ManagedFields = storedService.ManagedFields
		for key, value := range storedService.Annotations {
			if _, present := newService.Annotations[key]; !present {
				newService.Annotations[key] = value
			}
		}
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newService); err != nil {
			logger.Error(err, "Unable to patch redis service with comparison object")
			return err
		}
		logger.Info("Syncing Redis service with defined properties")
		return updateService(namespace, newService)
	}
	logger.Info("Redis service is already in-sync")
	return nil
}
