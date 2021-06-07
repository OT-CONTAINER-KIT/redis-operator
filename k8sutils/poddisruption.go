package k8sutils

import (

	// "github.com/google/go-cmp/cmp"
	"context"

	policyv1beta1 "k8s.io/api/policy/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	redisv1beta1 "redis-operator/api/v1beta1"
)

// CreateRedisSlave will create a Redis Slave
func CreatePodDisruptionBudget(cr *redisv1beta1.Redis, role string, replicas int32) {
	reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
	client := GenerateK8sClient()

	pdb := &policyv1beta1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: cr.ObjectMeta.Name + "-" + role,
		},
		Spec: policyv1beta1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{Type: intstr.Int, IntVal: int32(replicas)},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"instance": cr.ObjectMeta.Name, "role": role},
			},
		},
	}
	AddOwnerRefToObject(pdb, AsOwner(cr))

	ExistingPDB, err := GenerateK8sClient().PolicyV1beta1().PodDisruptionBudgets(cr.Namespace).Get(context.TODO(), pdb.Name, metav1.GetOptions{})
	if err != nil {
		reqLogger.Info("Creating redis PodDisruptionBudget", "Redis.Name", cr.ObjectMeta.Name+"-"+role)
		_, err := client.PolicyV1beta1().PodDisruptionBudgets(cr.Namespace).Create(context.TODO(), pdb, metav1.CreateOptions{})
		if err != nil {
			reqLogger.Error(err, "Failed in creating PDB for "+cr.ObjectMeta.Name)
		}
	} else {
		// Check to see if we actually NEED to update the PDB
		if apiequality.Semantic.DeepDerivative(ExistingPDB.Spec, pdb.Spec) {
			reqLogger.Info("Updating redis PodDisruptionBudget", "Redis.Name", ExistingPDB.Name)
			// Update Spec
			ExistingPDB.Spec = pdb.Spec
			_, err := client.PolicyV1beta1().PodDisruptionBudgets(cr.Namespace).Update(context.TODO(), ExistingPDB, metav1.UpdateOptions{})
			if err != nil {
				reqLogger.Error(err, "Failed in updating PDB for "+cr.ObjectMeta.Name)
			}
		}
	}

}
