package k8sutils

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta1"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	redisClusterPort = 16379
)

var (
	protocol         = corev1.ProtocolTCP
	networkPortRedis = networkv1.NetworkPolicyPort{
		Protocol: &protocol,
		Port: &intstr.IntOrString{
			IntVal: redisPort,
		},
	}
	networkPortRedisCluster = networkv1.NetworkPolicyPort{
		Protocol: &protocol,
		Port: &intstr.IntOrString{
			IntVal: redisClusterPort,
		},
	}
)

func generateNetworkPolicyDef(networkPolicyMeta metav1.ObjectMeta, config []v1beta1.NetworkPolicyConfigs) *networkv1.NetworkPolicy {
	networkPolicy := &networkv1.NetworkPolicy{
		ObjectMeta: networkPolicyMeta,
		Spec: networkv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: networkPolicyMeta.Labels,
			},
			Ingress: getNetworkPolicyIngressConfiguration(
				networkPolicyMeta,
				config,
			),
			Egress: []networkv1.NetworkPolicyEgressRule{
				{
					To: []networkv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: networkPolicyMeta.Labels,
							},
						},
					},
					Ports: []networkv1.NetworkPolicyPort{
						networkPortRedis,
						networkPortRedisCluster,
					},
				},
			},
			PolicyTypes: []networkv1.PolicyType{
				networkv1.PolicyTypeIngress,
				networkv1.PolicyTypeEgress,
			},
		},
	}

	return networkPolicy
}

func getNetworkPolicyIngressConfiguration(
	networkPolicyMeta metav1.ObjectMeta,
	configs []v1beta1.NetworkPolicyConfigs,
) []networkv1.NetworkPolicyIngressRule {
	ingresses := []networkv1.NetworkPolicyIngressRule{
		{
			From: []networkv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: networkPolicyMeta.Labels,
					},
				},
			},
			Ports: []networkv1.NetworkPolicyPort{
				networkPortRedis,
				networkPortRedisCluster,
			},
		},
	}

	if configs != nil {
		for _, config := range configs {
			ingresses = append(ingresses, networkv1.NetworkPolicyIngressRule{
				From: []networkv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: config.ExternalComponentMatchLabels,
						},
					},
				},
				Ports: []networkv1.NetworkPolicyPort{
					networkPortRedis,
				},
			})
		}
	}

	return ingresses
}

func createNetworkPolicy(namespace string, networkpolicy *networkv1.NetworkPolicy) error {
	logger := networkPolicyLogger(namespace, networkpolicy.Name)
	_, err := generateK8sClient().NetworkingV1().NetworkPolicies(namespace).Create(context.TODO(), networkpolicy, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "NetworkPolicy creation failed")
		return err
	}
	logger.Info("NetworkPolicy creation is successful")
	return nil
}

func updateNetworkPolicy(namespace string, networkpolicy *networkv1.NetworkPolicy) error {
	logger := networkPolicyLogger(namespace, networkpolicy.Name)
	_, err := generateK8sClient().NetworkingV1().NetworkPolicies(namespace).Update(context.TODO(), networkpolicy, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "NetworkPolicy update failed")
		return err
	}
	logger.Info("NetworkPolicy updated successfully")
	return nil
}

func getNetworkPolicy(namespace string, networkpolicy string) (*networkv1.NetworkPolicy, error) {
	logger := networkPolicyLogger(namespace, networkpolicy)
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("NetworkPolicy", "networking.k8s.io/v1"),
	}
	networkPolicyInfo, err := generateK8sClient().NetworkingV1().NetworkPolicies(namespace).Get(context.TODO(), networkpolicy, getOpts)
	if err != nil {
		logger.Info("NetworkPolicy get action failed")
		return nil, err
	}
	logger.Info("NetworkPolicy get action is successful")
	return networkPolicyInfo, nil
}

func networkPolicyLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.NetworkPolicy.Namespace", namespace, "Request.NetworkPolicy.Name", name)
	return reqLogger
}

func CreateOrUpdateNetworkPolicy(namespace string, networkPolicyMeta metav1.ObjectMeta, ownerDef metav1.OwnerReference, config []v1beta1.NetworkPolicyConfigs) error {
	logger := networkPolicyLogger(namespace, networkPolicyMeta.Name)
	networkPolicyDef := generateNetworkPolicyDef(networkPolicyMeta, config)
	storedNetworkPolicy, err := getNetworkPolicy(namespace, networkPolicyMeta.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(networkPolicyDef); err != nil {
				logger.Error(err, "Unable to patch network policy with compare annotations")
			}
			return createNetworkPolicy(namespace, networkPolicyDef)
		}
		return err
	}
	return patchNetworkPolicy(storedNetworkPolicy, networkPolicyDef, namespace)
}

func patchNetworkPolicy(storedNetworkPolicy *networkv1.NetworkPolicy, newNetworkPolicy *networkv1.NetworkPolicy, namespace string) error {
	logger := networkPolicyLogger(namespace, storedNetworkPolicy.Name)

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
		updateNetworkPolicy(namespace, newNetworkPolicy)
	}
	logger.Info("NetworkPolicy already in sync")
	return nil
}
