package k8sutils

import (
	"context"
	"github.com/banzaicloud/k8s-objectmatcher"
	corev1 "k8s.io/api/core/v1"
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

type Service interface {
	CreateOrUpdateService(namespace string, serviceMeta metav1.ObjectMeta, labels map[string]string) error
}

// generateHeadlessServiceDef generates service definition for headless service
func generateHeadlessServiceDef(serviceMeta metav1.ObjectMeta, labels map[string]string) *corev1.Service {
	service := &corev1.Service{
		TypeMeta:   generateMetaInformation("Service", "core/v1"),
		ObjectMeta: serviceMeta,
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "redis-client",
					Port:       portNumber,
					TargetPort: intstr.FromInt(int(redisPort)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// generateServiceDef generates service definition for Redis
func generateServiceDef(serviceMeta metav1.ObjectMeta, labels map[string]string, k8sServiceType string, enableMetrics bool) {
	service := &corev1.Service{
		TypeMeta:   generateMetaInformation("Service", "core/v1"),
		ObjectMeta: serviceMeta,
		Spec: corev1.ServiceSpec{
			Type:     generateServiceType(k8sServiceType),
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "redis-client",
					Port:       portNumber,
					TargetPort: intstr.FromInt(int(portNumber)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	if enableMetrics {
		service.Spec.Ports = append(service.Spec.Ports, enableMetricsPort())
	}
	return service
}

// enableMetricsPort will enable the metrics for Redis service
func enableMetricsPort() *corev1.ServicePort {
	return corev1.ServicePort{
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
func createService(namespace, service *corev1.Service) error {
	log := serviceLogger(namespace, service.Name)
	_, err := generateK8sClient().CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		log.Error(err, "Redis service creation is failed")
		return err
	}
	log.Info("Redis service creation is successful")
	return nil
}

// updateService is a method to update service is Kubernetes
func updateService(namespace, service *corev1.Service) error {
	log := serviceLogger(namespace, service.Name)
	_, err := generateK8sClient().CoreV1().Services(namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		log.Error(err, "Redis service updation is failed")
		return err
	}
	log.Info("Redis service updation is successful")
	return nil
}

// getService is a method to get service is Kubernetes
func getService(namespace, service string) (*corev1.Service, error) {
	log := serviceLogger(namespace, service.Name)
	serviceInfo, err := generateK8sClient().CoreV1().Services(namespace).Get(context.TODO(), service, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Redis service get action is failed")
		return err
	}
	log.Info("Redis service get action is successful")
	return serviceInfo, nil
}

func serviceLogger(namespace string, name string) {
	reqLogger := log.WithValues("Request.Service.Namespace", namespace, "Request.Service.Name")
	return reqLogger
}

func CreateOrUpdateService(namespace string, serviceMeta metav1.ObjectMeta, labels map[string]string) error {
	log := serviceLogger(namespace, serviceMeta.Name)
	storedService, err := getService(namespace, serviceMeta.Name)
	serviceDef := generateHeadlessServiceDef(serviceMeta, labels)
	if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(serviceDef); err != nil {
		log.Error(err, "Unable to patch redis service with comparison object")
		return err
	}
	if err != nil {
		if errors.IsNotFound(err) {
			return createService(namespace, serviceDef)
		}
		return err
	}
	patchResult, err := patch.DefaultPatchMaker.Calculate(storedService, serviceDef)
	if err != nil {
		log.Error(err, "Unable to patch redis service with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(serviceDef); err != nil {
			log.Error(err, "Unable to patch redis service with comparison object")
			return err
		}
		return updateService(namespace, serviceDef)
	}
}
