package k8sutils

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	redisv1beta1 "redis-operator/api/v1beta1"
)

const (
	redisPort = 6379
)

// ServiceInterface is the interface to pass service information accross methods
type ServiceInterface struct {
	ExistingService      *corev1.Service
	NewServiceDefinition *corev1.Service
	ServiceType          string
}

// GenerateHeadlessServiceDef generate service definition
func GenerateHeadlessServiceDef(cr *redisv1beta1.Redis, labels map[string]string, portNumber int32, role string, serviceName string, clusterIP string) *corev1.Service {
	var redisExporterPort int32 = 9121
	service := &corev1.Service{
		TypeMeta:   GenerateMetaInformation("Service", "core/v1"),
		ObjectMeta: GenerateObjectMetaInformation(serviceName, cr.Namespace, labels, GenerateServiceAnots()),
		Spec: corev1.ServiceSpec{
			ClusterIP: clusterIP,
			Selector:  labels,
			Ports: []corev1.ServicePort{
				{
					Name:       cr.ObjectMeta.Name + "-" + role,
					Port:       portNumber,
					TargetPort: intstr.FromInt(int(portNumber)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	if !cr.Spec.RedisExporter.Enabled {
		service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
			Name:       "redis-exporter",
			Port:       redisExporterPort,
			TargetPort: intstr.FromInt(int(redisExporterPort)),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	AddOwnerRefToObject(service, AsOwner(cr))
	return service
}

// GenerateServiceDef generate service definition
func GenerateServiceDef(cr *redisv1beta1.Redis, labels map[string]string, portNumber int32, role string, serviceName string, typeService string) *corev1.Service {
	var redisExporterPort int32 = 9121
	var serviceType corev1.ServiceType

	if typeService == "LoadBalancer" {
		serviceType = corev1.ServiceTypeLoadBalancer
	} else if typeService == "NodePort" {
		serviceType = corev1.ServiceTypeNodePort
	} else {
		serviceType = corev1.ServiceTypeClusterIP
	}

	service := &corev1.Service{
		TypeMeta:   GenerateMetaInformation("Service", "core/v1"),
		ObjectMeta: GenerateObjectMetaInformation(serviceName, cr.Namespace, labels, GenerateServiceAnots()),
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       cr.ObjectMeta.Name + "-" + role,
					Port:       portNumber,
					TargetPort: intstr.FromInt(int(portNumber)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	if !cr.Spec.RedisExporter.Enabled {
		service.Spec.Ports = append(service.Spec.Ports, corev1.ServicePort{
			Name:       "redis-exporter",
			Port:       redisExporterPort,
			TargetPort: intstr.FromInt(int(redisExporterPort)),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	AddOwnerRefToObject(service, AsOwner(cr))
	return service
}

// CreateMasterHeadlessService creates master headless service
func CreateMasterHeadlessService(cr *redisv1beta1.Redis) {
	labels := map[string]string{
		"app":  cr.ObjectMeta.Name + "-master",
		"role": "master",
	}
	serviceDefinition := GenerateHeadlessServiceDef(cr, labels, int32(redisPort), "master", cr.ObjectMeta.Name+"-master-headless", "None")
	serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(context.TODO(), cr.ObjectMeta.Name+"-master-headless", metav1.GetOptions{})
	service := ServiceInterface{
		ExistingService:      serviceBody,
		NewServiceDefinition: serviceDefinition,
		ServiceType:          "master",
	}
	CompareAndCreateHeadlessService(cr, service, err)
}

// CreateMasterService creates different services for master
func CreateMasterService(cr *redisv1beta1.Redis) {
	labels := map[string]string{
		"app":  cr.ObjectMeta.Name + "-master",
		"role": "master",
	}
	serviceDefinition := GenerateServiceDef(cr, labels, int32(redisPort), "master", cr.ObjectMeta.Name+"-master", cr.Spec.Master.Service.Type)
	serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(context.TODO(), cr.ObjectMeta.Name+"-master", metav1.GetOptions{})
	service := ServiceInterface{
		ExistingService:      serviceBody,
		NewServiceDefinition: serviceDefinition,
		ServiceType:          "master",
	}
	CompareAndCreateService(cr, service, err)
}

// CreateSlaveHeadlessService creates slave headless service
func CreateSlaveHeadlessService(cr *redisv1beta1.Redis) {
	labels := map[string]string{
		"app":  cr.ObjectMeta.Name + "-slave",
		"role": "slave",
	}
	serviceDefinition := GenerateHeadlessServiceDef(cr, labels, int32(redisPort), "slave", cr.ObjectMeta.Name+"-slave-headless", "None")
	serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(context.TODO(), cr.ObjectMeta.Name+"-slave-headless", metav1.GetOptions{})
	service := ServiceInterface{
		ExistingService:      serviceBody,
		NewServiceDefinition: serviceDefinition,
		ServiceType:          "slave",
	}
	CompareAndCreateHeadlessService(cr, service, err)
}

// CreateSlaveService creates different services for slave
func CreateSlaveService(cr *redisv1beta1.Redis) {
	labels := map[string]string{
		"app":  cr.ObjectMeta.Name + "-slave",
		"role": "slave",
	}
	serviceDefinition := GenerateServiceDef(cr, labels, int32(redisPort), "slave", cr.ObjectMeta.Name+"-slave", cr.Spec.Slave.Service.Type)
	serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(context.TODO(), cr.ObjectMeta.Name+"-slave", metav1.GetOptions{})
	service := ServiceInterface{
		ExistingService:      serviceBody,
		NewServiceDefinition: serviceDefinition,
		ServiceType:          "slave",
	}
	CompareAndCreateService(cr, service, err)
}

// CreateStandaloneService creates redis standalone service
func CreateStandaloneService(cr *redisv1beta1.Redis) {
	labels := map[string]string{
		"app":  cr.ObjectMeta.Name + "-" + "standalone",
		"role": "standalone",
	}
	serviceDefinition := GenerateServiceDef(cr, labels, int32(redisPort), "standalone", cr.ObjectMeta.Name, cr.Spec.Service.Type)
	serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(context.TODO(), cr.ObjectMeta.Name, metav1.GetOptions{})

	service := ServiceInterface{
		ExistingService:      serviceBody,
		NewServiceDefinition: serviceDefinition,
		ServiceType:          "standalone",
	}
	CompareAndCreateService(cr, service, err)
}

// CreateStandaloneHeadlessService creates redis standalone service
func CreateStandaloneHeadlessService(cr *redisv1beta1.Redis) {
	labels := map[string]string{
		"app":  cr.ObjectMeta.Name + "-" + "standalone",
		"role": "standalone",
	}
	serviceDefinition := GenerateHeadlessServiceDef(cr, labels, int32(redisPort), "standalone", cr.ObjectMeta.Name+"-headless", "None")
	serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(context.TODO(), cr.ObjectMeta.Name+"-headless", metav1.GetOptions{})

	service := ServiceInterface{
		ExistingService:      serviceBody,
		NewServiceDefinition: serviceDefinition,
		ServiceType:          "standalone",
	}
	CompareAndCreateHeadlessService(cr, service, err)
}

// CompareAndCreateService compares and creates service
func CompareAndCreateService(cr *redisv1beta1.Redis, service ServiceInterface, err error) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)

	if err != nil {
		reqLogger.Info("Creating redis service", "Redis.Name", cr.ObjectMeta.Name+"-"+service.ServiceType, "Service.Type", service.ServiceType)
		_, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Create(context.TODO(), service.NewServiceDefinition, metav1.CreateOptions{})
		if err != nil {
			reqLogger.Error(err, "Failed in creating service for redis")
		}
	}

	if service.ExistingService != nil {
		if service.ExistingService.Spec.Type != service.NewServiceDefinition.Spec.Type {
			existingService := service.ExistingService
			existingService.Spec.Type = service.NewServiceDefinition.Spec.Type
			if existingService.ObjectMeta.Name != "" && existingService != nil {
				reqLogger.Info("Service type has been updated for the service", "Redis.Name", cr.ObjectMeta.Name+"-"+service.ServiceType, "Service.Type", service.ServiceType)
				_, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Update(context.TODO(), existingService, metav1.UpdateOptions{})
				if err != nil {
					reqLogger.Error(err, "Failed in updating service for redis")
				}
			}
		}
	}
}

// CompareAndCreateService compares and creates service
func CompareAndCreateHeadlessService(cr *redisv1beta1.Redis, service ServiceInterface, err error) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)

	if err != nil {
		reqLogger.Info("Creating redis service", "Redis.Name", cr.ObjectMeta.Name+"-"+service.ServiceType, "Service.Type", service.ServiceType)
		_, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Create(context.TODO(), service.NewServiceDefinition, metav1.CreateOptions{})
		if err != nil {
			reqLogger.Error(err, "Failed in creating service for redis")
		}
	}
}
