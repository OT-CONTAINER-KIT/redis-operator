package otmachinery

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	redisv1alpha1 "redis-operator/pkg/apis/redis/v1alpha1"
	"strconv"
)

const (
	redisPort = 6379
)

type ServiceInterface struct {
	ExistingService      *corev1.Service
	NewServiceDefinition *corev1.Service
	ServiceType          string
}

// GenerateServiceDef generate service definition
func GenerateServiceDef(cr *redisv1alpha1.Redis, labels map[string]string, portNumber int32, role string, serviceName string, clusterIP string) *corev1.Service {
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
	if cr.Spec.RedisExporter != false {
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
func CreateMasterHeadlessService(cr *redisv1alpha1.Redis) {
	labels := map[string]string{
		"app":  cr.ObjectMeta.Name + "-master",
		"role": "master",
	}
	serviceDefinition := GenerateServiceDef(cr, labels, int32(redisPort), "master", cr.ObjectMeta.Name+"-master", "None")
	serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(cr.ObjectMeta.Name+"-master", metav1.GetOptions{})
	service := ServiceInterface{
		ExistingService:      serviceBody,
		NewServiceDefinition: serviceDefinition,
		ServiceType:          "master",
	}
	CompareAndCreateService(cr, service, err)
}

// CreateMasterService creates different services for master
func CreateMasterService(cr *redisv1alpha1.Redis) {
	masterReplicas := cr.Spec.Size
	for serviceCount := 0; serviceCount <= int(*masterReplicas)-1; serviceCount++ {
		labels := map[string]string{
			"app":                                cr.ObjectMeta.Name + "-master",
			"role":                               "master",
			"statefulset.kubernetes.io/pod-name": cr.ObjectMeta.Name + "-master-" + strconv.Itoa(serviceCount),
		}
		serviceDefinition := GenerateServiceDef(cr, labels, int32(redisPort), "master", cr.ObjectMeta.Name+"-master-"+strconv.Itoa(serviceCount), "None")
		serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(cr.ObjectMeta.Name+"-master-"+strconv.Itoa(serviceCount), metav1.GetOptions{})
		service := ServiceInterface{
			ExistingService:      serviceBody,
			NewServiceDefinition: serviceDefinition,
			ServiceType:          "master",
		}
		CompareAndCreateService(cr, service, err)
	}
}

// CreateSlaveHeadlessService creates slave headless service
func CreateSlaveHeadlessService(cr *redisv1alpha1.Redis) {
	labels := map[string]string{
		"app":  cr.ObjectMeta.Name + "-slave",
		"role": "slave",
	}
	serviceDefinition := GenerateServiceDef(cr, labels, int32(redisPort), "slave", cr.ObjectMeta.Name+"-slave", "None")
	serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(cr.ObjectMeta.Name+"-slave", metav1.GetOptions{})
	service := ServiceInterface{
		ExistingService:      serviceBody,
		NewServiceDefinition: serviceDefinition,
		ServiceType:          "slave",
	}
	CompareAndCreateService(cr, service, err)
}

// CreateSlaveService creates different services for slave
func CreateSlaveService(cr *redisv1alpha1.Redis) {
	slaveReplicas := cr.Spec.Size
	for serviceCount := 0; serviceCount <= int(*slaveReplicas)-1; serviceCount++ {
		labels := map[string]string{
			"app":                                cr.ObjectMeta.Name + "-slave",
			"role":                               "slave",
			"statefulset.kubernetes.io/pod-name": cr.ObjectMeta.Name + "-slave-" + strconv.Itoa(serviceCount),
		}
		serviceDefinition := GenerateServiceDef(cr, labels, int32(redisPort), "slave", cr.ObjectMeta.Name+"-slave-"+strconv.Itoa(serviceCount), "None")
		serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(cr.ObjectMeta.Name+"-slave-"+strconv.Itoa(serviceCount), metav1.GetOptions{})
		service := ServiceInterface{
			ExistingService:      serviceBody,
			NewServiceDefinition: serviceDefinition,
			ServiceType:          "slave",
		}
		CompareAndCreateService(cr, service, err)
	}
}

// CreateStandaloneService creates redis standalone service
func CreateStandaloneService(cr *redisv1alpha1.Redis) {
	labels := map[string]string{
		"app":  cr.ObjectMeta.Name + "-" + "standalone",
		"role": "standalone",
	}
	serviceDefinition := GenerateServiceDef(cr, labels, int32(redisPort), "standalone", cr.ObjectMeta.Name, "None")
	serviceBody, err := GenerateK8sClient().CoreV1().Services(cr.Namespace).Get(cr.ObjectMeta.Name, metav1.GetOptions{})

	service := ServiceInterface{
		ExistingService:      serviceBody,
		NewServiceDefinition: serviceDefinition,
		ServiceType:          "standalone",
	}
	CompareAndCreateService(cr, service, err)
}

// CompareAndCreateService compares and creates service
func CompareAndCreateService(cr *redisv1alpha1.Redis, service ServiceInterface, err error) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)

	if err != nil {
		reqLogger.Info("Creating redis service", "Redis.Name", cr.ObjectMeta.Name+"-"+service.ServiceType, "Service.Type", service.ServiceType)
		GenerateK8sClient().CoreV1().Services(cr.Namespace).Create(service.NewServiceDefinition)
	} else if service.ExistingService != service.NewServiceDefinition {
		reqLogger.Info("Reconciling redis service", "Redis.Name", cr.ObjectMeta.Name+"-"+service.ServiceType, "Service.Type", service.ServiceType)
		GenerateK8sClient().CoreV1().Services(cr.Namespace).Update(service.NewServiceDefinition)
	} else {
		reqLogger.Info("Redis service is in sync", "Redis.Name", cr.ObjectMeta.Name+"-"+service.ServiceType, "Service.Type", service.ServiceType)
	}
}
