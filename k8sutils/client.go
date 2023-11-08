package k8sutils

import (
	// custom "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sConfigProvider = func() (*rest.Config, error)

// generateK8sClient create client for kubernetes
func generateK8sClient(configProvider K8sConfigProvider) (kubernetes.Interface, error) {
	config, err := configProvider()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// generateK8sClient create Dynamic client for kubernetes
func generateK8sDynamicClient(configProvider K8sConfigProvider) (dynamic.Interface, error) {
	config, err := configProvider()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}

// generateK8sConfig will load the kube config file
func generateK8sConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here
	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return kubeConfig.ClientConfig()
}
