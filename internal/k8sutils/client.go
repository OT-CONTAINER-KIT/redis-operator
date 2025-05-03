package k8sutils

import (
	// custom "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sConfigProvider = func() (*rest.Config, error)

// GenerateK8sClient create client for kubernetes
func GenerateK8sClient(configProvider K8sConfigProvider) (kubernetes.Interface, error) {
	config, err := configProvider()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// GenerateK8sClient create Dynamic client for kubernetes
func GenerateK8sDynamicClient(configProvider K8sConfigProvider) (dynamic.Interface, error) {
	config, err := configProvider()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}

// GenerateK8sConfig will load the kube config file
func GenerateK8sConfig() K8sConfigProvider {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here
	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return kubeConfig.ClientConfig
}
