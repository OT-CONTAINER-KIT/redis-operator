package k8sutils

import (
	"context"

	"github.com/OT-CONTAINER-KIT/redis-operator/pkg/util"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	redisPort             = 6379
	sentinelPort          = 26379
	redisExporterPort     = 9121
	redisExporterPortName = "redis-exporter"
)

var serviceType corev1.ServiceType

// exporterPortProvider return the exporter port if bool is true
type exporterPortProvider func() (port int, enable bool)

var disableMetrics exporterPortProvider = func() (int, bool) {
	return 0, false
}

// generateServiceDef generates service definition for Redis
func generateServiceDef(serviceMeta metav1.ObjectMeta, epp exporterPortProvider, ownerDef metav1.OwnerReference, headless bool, serviceType string, port int, extra ...corev1.ServicePort) *corev1.Service {
	var PortName string
	if serviceMeta.Labels["role"] == "sentinel" {
		PortName = "sentinel-client"
	} else {
		PortName = "redis-client"
	}
	service := &corev1.Service{
		TypeMeta:   generateMetaInformation("Service", "v1"),
		ObjectMeta: serviceMeta,
		Spec: corev1.ServiceSpec{
			Type:      generateServiceType(serviceType),
			ClusterIP: "",
			Selector:  util.CopyMap(serviceMeta.GetLabels()),
			Ports: []corev1.ServicePort{
				{
					Name:       PortName,
					Port:       int32(port),
					TargetPort: intstr.FromInt(port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	if headless {
		service.Spec.ClusterIP = "None"
	}
	if exporterPort, ok := epp(); ok {
		redisExporterService := enableMetricsPort(exporterPort)
		service.Spec.Ports = append(service.Spec.Ports, *redisExporterService)
	}
	if len(extra) > 0 {
		service.Spec.Ports = append(service.Spec.Ports, extra...)
	}

	AddOwnerRefToObject(service, ownerDef)
	return service
}

// enableMetricsPort will enable the metrics for Redis service
func enableMetricsPort(port int) *corev1.ServicePort {
	return &corev1.ServicePort{
		Name:       redisExporterPortName,
		Port:       int32(port),
		TargetPort: intstr.FromInt(port),
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
func createService(ctx context.Context, kusClient kubernetes.Interface, namespace string, service *corev1.Service) error {
	_, err := kusClient.CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "Redis service creation is failed")
		return err
	}
	log.FromContext(ctx).V(1).Info("Redis service creation is successful")
	return nil
}

// updateService is a method to update service is Kubernetes
func updateService(ctx context.Context, k8sClient kubernetes.Interface, namespace string, service *corev1.Service) error {
	_, err := k8sClient.CoreV1().Services(namespace).Update(context.TODO(), service, metav1.UpdateOptions{})
	if err != nil {
		log.FromContext(ctx).Error(err, "Redis service update failed")
		return err
	}
	log.FromContext(ctx).V(1).Info("Redis service updated successfully")
	return nil
}

// getService is a method to get service is Kubernetes
func getService(ctx context.Context, k8sClient kubernetes.Interface, namespace string, name string) (*corev1.Service, error) {
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("Service", "v1"),
	}
	serviceInfo, err := k8sClient.CoreV1().Services(namespace).Get(context.TODO(), name, getOpts)
	if err != nil {
		log.FromContext(ctx).V(1).Info("Redis service get action is failed")
		return nil, err
	}
	log.FromContext(ctx).V(1).Info("Redis service get action is successful")
	return serviceInfo, nil
}

// CreateOrUpdateService method will create or update Redis service
func CreateOrUpdateService(ctx context.Context, namespace string, serviceMeta metav1.ObjectMeta, ownerDef metav1.OwnerReference, epp exporterPortProvider, headless bool, serviceType string, port int, cl kubernetes.Interface, extra ...corev1.ServicePort) error {
	serviceDef := generateServiceDef(serviceMeta, epp, ownerDef, headless, serviceType, port, extra...)
	storedService, err := getService(ctx, cl, namespace, serviceMeta.GetName())
	if err != nil {
		if errors.IsNotFound(err) {
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(serviceDef); err != nil { //nolint:gocritic
				log.FromContext(ctx).Error(err, "Unable to patch redis service with compare annotations")
			}
			return createService(ctx, cl, namespace, serviceDef)
		}
		return err
	}
	return patchService(ctx, storedService, serviceDef, namespace, cl)
}

// patchService will patch Redis Kubernetes service
func patchService(ctx context.Context, storedService *corev1.Service, newService *corev1.Service, namespace string, cl kubernetes.Interface) error {
	// We want to try and keep this atomic as possible.
	newService.ResourceVersion = storedService.ResourceVersion
	newService.CreationTimestamp = storedService.CreationTimestamp
	newService.ManagedFields = storedService.ManagedFields
	newService.Finalizers = storedService.Finalizers

	if newService.Spec.Type == generateServiceType("ClusterIP") {
		newService.Spec.ClusterIP = storedService.Spec.ClusterIP
	}

	patchResult, err := patch.DefaultPatchMaker.Calculate(storedService, newService,
		patch.IgnoreStatusFields(),
		patch.IgnoreField("kind"),
		patch.IgnoreField("apiVersion"),
	)
	if err != nil {
		log.FromContext(ctx).Error(err, "Unable to patch redis service with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		log.FromContext(ctx).V(1).Info("Changes in service Detected, Updating...", "patch", string(patchResult.Patch))

		util.MergePreservingExistingKeys(newService.Annotations, storedService.Annotations)
		// util.MergePreservingExistingKeys(newService.Labels, storedService.Labels)
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newService); err != nil {
			log.FromContext(ctx).Error(err, "Unable to patch redis service with comparison object")
			return err
		}
		log.FromContext(ctx).V(1).Info("Syncing Redis service with defined properties")
		return updateService(ctx, cl, namespace, newService)
	}
	log.FromContext(ctx).V(1).Info("Redis service is already in-sync")
	return nil
}
