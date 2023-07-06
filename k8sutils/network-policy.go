package k8sutils

import (
	"context"

	"k8s.io/client-go/kubernetes"

	"github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta1"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Defining Ports to be allowed for communication
var (
	protocol = corev1.ProtocolTCP
	ports    = []networkv1.NetworkPolicyPort{
		{
			Protocol: &protocol,
			Port: &intstr.IntOrString{
				IntVal: redisPort,
			},
		},
		{
			Protocol: &protocol,
			Port: &intstr.IntOrString{
				IntVal: redisExporterPort,
			},
		},
	}

	sentinelPorts = []networkv1.NetworkPolicyPort{
		{
			Protocol: &protocol,
			Port: &intstr.IntOrString{
				IntVal: sentinelPort,
			},
		},
		{
			Protocol: &protocol,
			Port: &intstr.IntOrString{
				IntVal: redisExporterPort,
			},
		},
	}
)

// networkPolicyTemplate makes the network policy for redis along with ingress and egress rules for application connectivity
func networkPolicyTemplate(labelSelector metav1.LabelSelector, port []networkv1.NetworkPolicyPort, networkPolicyMeta metav1.ObjectMeta, networkPolicy v1beta1.NetworkPolicy) *networkv1.NetworkPolicy {

	// Defining default Ingress and Egress rules
	ingress := []networkv1.NetworkPolicyIngressRule{
		{
			From: []networkv1.NetworkPolicyPeer{
				{
					PodSelector: &labelSelector,
				},
			},
			Ports: ports,
		},
	}

	egress := []networkv1.NetworkPolicyEgressRule{}

	// Checking if ingressRules are not empty and adding them to the rules
	if networkPolicy.Ingress != nil {
		ingress = append(ingress, networkPolicy.Ingress...)
	}

	// Checking if egressRules are not empty and adding them to the rules
	if networkPolicy.Egress != nil {
		egress = append(egress, networkPolicy.Egress...)
	}

	networkPolicyNew := &networkv1.NetworkPolicy{
		ObjectMeta: networkPolicyMeta,
		Spec: networkv1.NetworkPolicySpec{
			PodSelector: labelSelector,
			Ingress:     ingress,
			Egress:      egress,
			PolicyTypes: []networkv1.PolicyType{
				networkv1.PolicyTypeIngress,
				networkv1.PolicyTypeEgress,
			},
		},
	}
	return networkPolicyNew
}

// getNetworkPolicy will get the network policy
func getNetworkPolicy(client *kubernetes.Clientset, namespace string, networkpolicyName string) (*networkv1.NetworkPolicy, error) {
	logger := getNetworkPolicyLogger(namespace, networkpolicyName)
	networkPolicy, err := client.NetworkingV1().NetworkPolicies(namespace).Get(context.TODO(), networkpolicyName, metav1.GetOptions{})
	if err != nil {
		logger.Info("Failed to get network policy")
		return nil, err
	}
	logger.Info("Network policy retrieved successfully")
	return networkPolicy, nil
}

// patchNetworkPolicy will patch the network policy
func patchNetworkPolicy(client *kubernetes.Clientset, storedNetworkPolicy *networkv1.NetworkPolicy, newNetworkPolicy *networkv1.NetworkPolicy, namespace string) error {
	logger := getNetworkPolicyLogger(namespace, storedNetworkPolicy.Name)

	newNetworkPolicy.ResourceVersion = storedNetworkPolicy.ResourceVersion
	newNetworkPolicy.CreationTimestamp = storedNetworkPolicy.CreationTimestamp
	newNetworkPolicy.ManagedFields = storedNetworkPolicy.ManagedFields

	patchResult, err := patch.DefaultPatchMaker.Calculate(storedNetworkPolicy, newNetworkPolicy, patch.IgnoreStatusFields(), patch.IgnoreField("kind"), patch.IgnoreField("apiVersion"))
	if err != nil {
		logger.Error(err, "Unable to patch network policy with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		logger.Info("Changes in network policy detected, updating...", "patch", string(patchResult.Patch))

		for key, value := range storedNetworkPolicy.Annotations {
			if _, present := newNetworkPolicy.Annotations[key]; !present {
				newNetworkPolicy.Annotations[key] = value
			}
		}
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newNetworkPolicy); err != nil {
			logger.Error(err, "Unable to patch network policy with comparison object")
			return err
		}
		logger.Info("Syncing network policy with defined properties")
		updateNetworkPolicy(client, namespace, newNetworkPolicy)
	}
	logger.Info("NetworkPolicy already in sync")
	return nil
}

// createNetworkPolicy will create a network policy
func createNetworkPolicy(client *kubernetes.Clientset, namespace string, networkPolicy *networkv1.NetworkPolicy) error {
	logger := getNetworkPolicyLogger(namespace, networkPolicy.Name)
	_, err := client.NetworkingV1().NetworkPolicies(namespace).Create(context.TODO(), networkPolicy, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Network policy creation failed")
		return err
	}

	logger.Info("Network policy created successfully")
	return nil
}

// updateNetworkPolicy will update a network policy
func updateNetworkPolicy(client *kubernetes.Clientset, namespace string, networkPolicy *networkv1.NetworkPolicy) error {
	logger := getNetworkPolicyLogger(namespace, networkPolicy.Name)
	_, err := client.NetworkingV1().NetworkPolicies(namespace).Update(context.TODO(), networkPolicy, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Network policy updation failed")
		return err
	}

	logger.Info("Network policy updated successfully")
	return nil
}

// networkPolicyLogger will generate logging interface for networkPolicy
func getNetworkPolicyLogger(namespace string, name string) logr.Logger {
	return log.WithValues("Request.NetworkPolicy.Namespace", namespace, "Request.NetworkPolicy.Name", name)
}
