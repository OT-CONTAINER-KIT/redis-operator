package k8sutils

import (
	redisv1beta1 "redis-operator/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// generateMetaInformation generates the meta information
func generateMetaInformation(resourceKind string, apiVersion string) metav1.TypeMeta {
	return metav1.TypeMeta{
		Kind:       resourceKind,
		APIVersion: apiVersion,
	}
}

// generateObjectMetaInformation generates the object meta information
func generateObjectMetaInformation(name string, namespace string, labels map[string]string, annotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: annotations,
	}
}

// AddOwnerRefToObject adds the owner references to object
func AddOwnerRefToObject(obj metav1.Object, ownerRef metav1.OwnerReference) {
	obj.SetOwnerReferences(append(obj.GetOwnerReferences(), ownerRef))
}

// AsOwner generates and returns object refernece
func AsOwner(cr *redisv1beta1.Redis) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: cr.APIVersion,
		Kind:       cr.Kind,
		Name:       cr.Name,
		UID:        cr.UID,
		Controller: &trueVar,
	}
}

// redisAsOwner generates and returns object refernece
func redisAsOwner(cr *redisv1beta1.Redis) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: cr.APIVersion,
		Kind:       cr.Kind,
		Name:       cr.Name,
		UID:        cr.UID,
		Controller: &trueVar,
	}
}

// redisClusterAsOwner generates and returns object refernece
func redisClusterAsOwner(cr *redisv1beta1.RedisCluster) metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion: cr.APIVersion,
		Kind:       cr.Kind,
		Name:       cr.Name,
		UID:        cr.UID,
		Controller: &trueVar,
	}
}

// generateStatefulSetsAnots generates and returns statefulsets annotations
func generateStatefulSetsAnots() map[string]string {
	return map[string]string{
		"redis.opstreelabs.in": "true",
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "9121",
	}
}

// generateServiceAnots generates and returns service annotations
func generateServiceAnots() map[string]string {
	return map[string]string{
		"redis.opstreelabs.in": "true",
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "9121",
	}
}

// LabelSelectors generates object for label selection
func LabelSelectors(labels map[string]string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: labels}
}
