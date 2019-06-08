
package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Redis is the Schema for the redis API
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="Master",type="string",JSONPath=".status.master",description="Current master's Pod name"
// +kubebuilder:printcolumn:name="Current",type="integer",JSONPath=".status.replicas",description="Current number of Redis instances"
// +kubebuilder:printcolumn:name="Desired",type="integer",JSONPath=".spec.replicas",description="Desired number of Redis instances"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=."spec.replicas",statuspath=".status.replicas"
type Redis struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisSpec   `json:"spec,omitempty"`
	Status RedisStatus `json:"status,omitempty"`
}

// RedisSpec defines the desired state of Redis
// +k8s:openapi-gen=true
type RedisSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html

	// Replicas is a number of replicas in a Redis failover cluster
	// +kubebuilder:validation:Minimum=3
	Replicas *int32 `json:"replicas"`

	// Config allows to pass custom Redis configuration parameters
	Config   map[string]string `json:"config,omitempty"`
	Password Password          `json:"password,omitempty"`

	// Pod annotations
	Annotations map[string]string `json:"annotations,omitempty"`
	// Pod securityContext
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// Pod affinity
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// Pod tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// Pod priorityClassName
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// DataVolumeClaimTemplate for StatefulSet
	DataVolumeClaimTemplate corev1.PersistentVolumeClaim `json:"dataVolumeClaimTemplate,omitempty"`

	// Redis container specification
	Redis ContainerSpec `json:"redis"`

	// Exporter container specification
	Exporter ContainerSpec `json:"exporter,omitempty"`
}

// Password allows to refer to a Secret containing password for Redis
// Password should be strong enough. Passwords shorter than 8 characters
// composed of ASCII alphanumeric symbols will lead to a mild warning logged by the Operator.
// Please note that password hashes are added as annotations to Pods to enable
// password rotation. Hashes are generated using argon2id KDF.
// Changing the password in the referenced Secret will not trigger
// the rolling Statefulset upgrade automatically.
// However an event in regard to any objects owned by the Redis resource
// fired afterwards will trigger the rolling upgrade.
// Redis operator does not store the password internally and reads it
// from the Secret any time the Reconcile is called.
// Hence it will not be able to connect to Pods with the ``old'' password.
// In scenarios when persistence is turned off all the data will be lost
// during password rotation.
// +k8s:openapi-gen=true
type Password struct {
	// SecretKeyRef is a reference to the Secret in the same namespace containing the password.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef"`
}

// ContainerSpec allows to set some container-specific attributes
// +k8s:openapi-gen=true
type ContainerSpec struct {
	// Image is a standard path for a Container image
	Image string `json:"image"`
	// Resources describes the compute resource requirements
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// SecurityContext holds security configuration that will be applied to a container
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// RedisStatus defines the observed state of Redis
// +k8s:openapi-gen=true
type RedisStatus struct {
	// Replicas is the number of active Redis instances in the replication
	Replicas int `json:"replicas"`
	// Master is the current master's Pod name
	Master string `json:"master"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedisList contains a list of Redis resources
// +k8s:openapi-gen=true
type RedisList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	// +k8s:openapi-gen=false
	metav1.ListMeta `json:"metadata,omitempty"`
	// List of Redis resources
	Items []Redis `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Redis{}, &RedisList{})
}
