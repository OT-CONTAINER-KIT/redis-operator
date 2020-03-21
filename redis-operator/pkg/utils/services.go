package otmachinery

import (
	redisv1alpha1 "redis-operator/redis-operator/pkg/apis/redis/v1alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1 "k8s.io/api/core/v1"
)

// GenerateServiceDef generate service definition
func GenerateServiceDef(cr *redisv1alpha1.Redis, labels map[string]string, portNumber int32, role string, serviceName string) *corev1.Service{
	var redisExporterPort int32 = 9121
	service := &corev1.Service{
		TypeMeta:   GenerateMetaInformation("Service", "core/v1"),
		ObjectMeta:  GenerateObjectMetaInformation(serviceName, cr.Namespace, labels, GenerateServiceAnots()),
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
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
func CreateMasterHeadlessService(cr *redisv1alpha1.Redis, role string) *corev1.Service{
	labels := map[string]string{
		"app": cr.ObjectMeta.Name + "-" + role,
		"role": role,
	}
	return GenerateServiceDef(cr, labels, int32(6379), "master", cr.ObjectMeta.Name + "-" + role)
}

// CreateMasterService creates different services for master
func CreateMasterService(cr *redisv1alpha1.Redis, role string, statefulSet string) *corev1.Service{
	labels := map[string]string{
		"app": cr.ObjectMeta.Name + "-" + role,
		"role": role,
		"statefulset.kubernetes.io/pod-name": cr.ObjectMeta.Name + "-" + role + "-" + statefulSet,
	}
	return GenerateServiceDef(cr, labels, int32(6379), "master", cr.ObjectMeta.Name + "-" + role + "-" + statefulSet)
}

// CreateSlaveHeadlessService creates slave headless service
func CreateSlaveHeadlessService(cr *redisv1alpha1.Redis, role string) *corev1.Service{
	labels := map[string]string{
		"app": cr.ObjectMeta.Name + "-" + role,
		"role": role,
	}
	return GenerateServiceDef(cr, labels, int32(6379), "slave", cr.ObjectMeta.Name + "-" + role)
}

// CreateSlaveService creates different services for slave
func CreateSlaveService(cr *redisv1alpha1.Redis, role string, statefulSet string) *corev1.Service{
	labels := map[string]string{
		"app": cr.ObjectMeta.Name + "-" + role,
		"role": role,
		"statefulset.kubernetes.io/pod-name": cr.ObjectMeta.Name + "-" + role + "-" + statefulSet,
	}
	return GenerateServiceDef(cr, labels, int32(6379), "slave", cr.ObjectMeta.Name + "-" + role + "-" + statefulSet)
}
