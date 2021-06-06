package k8sutils

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// generateK8sClient create client for kubernetes
func generateK8sClient() *kubernetes.Clientset {
	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)
	return clientset
}
