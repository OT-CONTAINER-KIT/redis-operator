package k8sutils

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GenerateK8sClient create client for kubernetes
func GenerateK8sClient() *kubernetes.Clientset {
	config, err := GenerateK8sConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset
}

func GenerateK8sConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here
	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return kubeConfig.ClientConfig()
}
