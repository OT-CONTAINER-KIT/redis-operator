package testutil

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SkipReconcileTestConfig holds configuration for skip-reconcile tests
type SkipReconcileTestConfig struct {
	// Object is the custom resource object to test
	Object client.Object
	// SkipAnnotationKey is the annotation key for skip-reconcile
	SkipAnnotationKey string
	// StatefulSetName is the expected StatefulSet name
	StatefulSetName string
	// Namespace is the test namespace
	Namespace string
	// Timeout for Eventually assertions
	Timeout time.Duration
	// Interval for Eventually assertions
	Interval time.Duration
}

// RunSkipReconcileTest runs the skip-reconcile test logic
func RunSkipReconcileTest(k8sClient client.Client, config SkipReconcileTestConfig) {
	// Set skip-reconcile annotation to true
	annotations := config.Object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[config.SkipAnnotationKey] = "true"
	config.Object.SetAnnotations(annotations)

	Expect(k8sClient.Create(context.Background(), config.Object)).Should(Succeed())
	defer func() {
		Expect(k8sClient.Delete(context.Background(), config.Object)).Should(Succeed())
	}()

	By("verifying that no StatefulSet is created when skip-reconcile is true")
	sts := &appsv1.StatefulSet{}
	Consistently(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Name:      config.StatefulSetName,
			Namespace: config.Namespace,
		}, sts)
	}, time.Second*3, time.Millisecond*500).ShouldNot(Succeed())

	By("updating skip-reconcile annotation to false")
	// Get the updated object from cluster
	updatedObj := config.Object.DeepCopyObject().(client.Object)
	Expect(k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      config.Object.GetName(),
		Namespace: config.Namespace,
	}, updatedObj)).Should(Succeed())

	// Update the annotation
	annotations = updatedObj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[config.SkipAnnotationKey] = "false"
	updatedObj.SetAnnotations(annotations)
	Expect(k8sClient.Update(context.Background(), updatedObj)).Should(Succeed())

	By("verifying that StatefulSet is created after skip-reconcile is set to false")
	Eventually(func() error {
		return k8sClient.Get(context.Background(), types.NamespacedName{
			Name:      config.StatefulSetName,
			Namespace: config.Namespace,
		}, sts)
	}, config.Timeout, config.Interval).Should(Succeed())
}

// CreateTestObject creates a test object with the given name, namespace and annotations
func CreateTestObject(name, namespace string, annotations map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   namespace,
		Annotations: annotations,
	}
}
