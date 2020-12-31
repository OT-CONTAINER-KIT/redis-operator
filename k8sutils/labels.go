package k8sutils

import (
	redisv1beta1 "redis-operator/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateMetaInformation generates the meta information
func GenerateMetaInformation(resourceKind string, apiVersion string) metav1.TypeMeta {
	return metav1.TypeMeta{
		Kind:       resourceKind,
		APIVersion: apiVersion,
	}
}

// GenerateObjectMetaInformation generates the object meta information
func GenerateObjectMetaInformation(name string, namespace string, labels map[string]string, annotations map[string]string) metav1.ObjectMeta {
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

// GenerateStatefulSetsAnots generates and returns statefulsets annotations
func GenerateStatefulSetsAnots() map[string]string {
	return map[string]string{
		"redis.opstreelabs.in": "true",
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "9121",
	}
}

// GenerateServiceAnots generates and returns service annotations
func GenerateServiceAnots() map[string]string {
	return map[string]string{
		"redis.opstreelabs.in": "true",
		"prometheus.io/scrape": "true",
		"prometheus.io/port":   "9121",
	}
}

// GenerateSecretAnots generates and returns secrets annotations
func GenerateSecretAnots() map[string]string {
	return map[string]string{
		"redis.opstreelabs.in": "true",
	}
}

// LabelSelectors generates object for label selection
func LabelSelectors(labels map[string]string) *metav1.LabelSelector {
	return &metav1.LabelSelector{MatchLabels: labels}
}
