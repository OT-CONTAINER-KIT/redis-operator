package k8sutils

import (
	"context"
	"fmt"

	commonapi "github.com/OT-CONTAINER-KIT/redis-operator/api"
	redisv1beta2 "github.com/OT-CONTAINER-KIT/redis-operator/api/v1beta2"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// CreateRedisLeaderPodDisruptionBudget check and create a PodDisruptionBudget for Leaders
func ReconcileRedisPodDisruptionBudget(cr *redisv1beta2.RedisCluster, role string, pdbParams *commonapi.RedisPodDisruptionBudget, cl kubernetes.Interface) error {
	pdbName := cr.ObjectMeta.Name + "-" + role
	logger := pdbLogger(cr.Namespace, pdbName)
	if pdbParams != nil && pdbParams.Enabled {
		labels := getRedisLabels(cr.ObjectMeta.Name, cluster, role, cr.ObjectMeta.GetLabels())
		annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
		pdbMeta := generateObjectMetaInformation(pdbName, cr.Namespace, labels, annotations)
		pdbDef := generatePodDisruptionBudgetDef(cr, role, pdbMeta, cr.Spec.RedisLeader.PodDisruptionBudget)
		return CreateOrUpdatePodDisruptionBudget(pdbDef, cl)
	} else {
		// Check if one exists, and delete it.
		_, err := GetPodDisruptionBudget(cr.Namespace, pdbName, cl)
		if err == nil {
			return deletePodDisruptionBudget(cr.Namespace, pdbName, cl)
		} else if err != nil && errors.IsNotFound(err) {
			logger.V(1).Info("Reconciliation Successful, no PodDisruptionBudget Found.")
			// Its ok if its not found, as we're deleting anyway
			return nil
		}
		return err
	}
}

func ReconcileSentinelPodDisruptionBudget(cr *redisv1beta2.RedisSentinel, pdbParams *commonapi.RedisPodDisruptionBudget, cl kubernetes.Interface) error {
	pdbName := cr.ObjectMeta.Name + "-sentinel"
	logger := pdbLogger(cr.Namespace, pdbName)
	if pdbParams != nil && pdbParams.Enabled {
		labels := getRedisLabels(cr.ObjectMeta.Name, sentinel, "sentinel", cr.ObjectMeta.GetLabels())
		annotations := generateStatefulSetsAnots(cr.ObjectMeta, cr.Spec.KubernetesConfig.IgnoreAnnotations)
		pdbMeta := generateObjectMetaInformation(pdbName, cr.Namespace, labels, annotations)
		pdbDef := generateSentinelPodDisruptionBudgetDef(cr, "sentinel", pdbMeta, pdbParams)
		return CreateOrUpdatePodDisruptionBudget(pdbDef, cl)
	} else {
		// Check if one exists, and delete it.
		_, err := GetPodDisruptionBudget(cr.Namespace, pdbName, cl)
		if err == nil {
			return deletePodDisruptionBudget(cr.Namespace, pdbName, cl)
		} else if err != nil && errors.IsNotFound(err) {
			logger.V(1).Info("Reconciliation Successful, no PodDisruptionBudget Found.")
			// Its ok if its not found, as we're deleting anyway
			return nil
		}
		return err
	}
}

// generatePodDisruptionBudgetDef will create a PodDisruptionBudget definition
func generatePodDisruptionBudgetDef(cr *redisv1beta2.RedisCluster, role string, pdbMeta metav1.ObjectMeta, pdbParams *commonapi.RedisPodDisruptionBudget) *policyv1.PodDisruptionBudget {
	lblSelector := LabelSelectors(map[string]string{
		"app":  fmt.Sprintf("%s-%s", cr.ObjectMeta.Name, role),
		"role": role,
	})
	pdbTemplate := &policyv1.PodDisruptionBudget{
		TypeMeta:   generateMetaInformation("PodDisruptionBudget", "policy/v1"),
		ObjectMeta: pdbMeta,
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: lblSelector,
		},
	}
	if pdbParams.MinAvailable != nil {
		pdbTemplate.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: (*pdbParams.MinAvailable)}
	}
	if pdbParams.MaxUnavailable != nil {
		pdbTemplate.Spec.MaxUnavailable = &intstr.IntOrString{Type: intstr.Int, IntVal: *pdbParams.MaxUnavailable}
	}
	// If we don't have a value for either, assume quorum: (N/2)+1
	if pdbTemplate.Spec.MaxUnavailable == nil && pdbTemplate.Spec.MinAvailable == nil {
		pdbTemplate.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: (*cr.Spec.Size / 2) + 1}
	}
	AddOwnerRefToObject(pdbTemplate, redisClusterAsOwner(cr))
	return pdbTemplate
}

// generatePodDisruptionBudgetDef will create a PodDisruptionBudget definition
func generateSentinelPodDisruptionBudgetDef(cr *redisv1beta2.RedisSentinel, role string, pdbMeta metav1.ObjectMeta, pdbParams *commonapi.RedisPodDisruptionBudget) *policyv1.PodDisruptionBudget {
	lblSelector := LabelSelectors(map[string]string{
		"app":  fmt.Sprintf("%s-%s", cr.ObjectMeta.Name, role),
		"role": role,
	})
	pdbTemplate := &policyv1.PodDisruptionBudget{
		TypeMeta:   generateMetaInformation("PodDisruptionBudget", "policy/v1"),
		ObjectMeta: pdbMeta,
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: lblSelector,
		},
	}
	if pdbParams.MinAvailable != nil {
		pdbTemplate.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: *pdbParams.MinAvailable}
	}
	if pdbParams.MaxUnavailable != nil {
		pdbTemplate.Spec.MaxUnavailable = &intstr.IntOrString{Type: intstr.Int, IntVal: *pdbParams.MaxUnavailable}
	}
	// If we don't have a value for either, assume quorum: (N/2)+1
	if pdbTemplate.Spec.MaxUnavailable == nil && pdbTemplate.Spec.MinAvailable == nil {
		pdbTemplate.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: (*cr.Spec.Size / 2) + 1}
	}
	AddOwnerRefToObject(pdbTemplate, redisSentinelAsOwner(cr))
	return pdbTemplate
}

// CreateOrUpdateService method will create or update Redis service
func CreateOrUpdatePodDisruptionBudget(pdbDef *policyv1.PodDisruptionBudget, cl kubernetes.Interface) error {
	logger := pdbLogger(pdbDef.Namespace, pdbDef.Name)
	storedPDB, err := GetPodDisruptionBudget(pdbDef.Namespace, pdbDef.Name, cl)
	if err != nil {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(pdbDef); err != nil { //nolint
			logger.Error(err, "Unable to patch redis PodDisruptionBudget with comparison object")
			return err
		}
		if errors.IsNotFound(err) {
			return createPodDisruptionBudget(pdbDef.Namespace, pdbDef, cl)
		}
		return err
	}
	return patchPodDisruptionBudget(storedPDB, pdbDef, pdbDef.Namespace, cl)
}

// patchPodDisruptionBudget will patch Redis Kubernetes PodDisruptionBudgets
func patchPodDisruptionBudget(storedPdb *policyv1.PodDisruptionBudget, newPdb *policyv1.PodDisruptionBudget, namespace string, cl kubernetes.Interface) error {
	logger := pdbLogger(namespace, storedPdb.Name)
	// We want to try and keep this atomic as possible.
	newPdb.ResourceVersion = storedPdb.ResourceVersion
	newPdb.CreationTimestamp = storedPdb.CreationTimestamp
	newPdb.ManagedFields = storedPdb.ManagedFields

	// newPdb.Kind = "PodDisruptionBudget"
	// newPdb.APIVersion = "policy/v1"
	storedPdb.Kind = "PodDisruptionBudget"
	storedPdb.APIVersion = "policy/v1"

	patchResult, err := patch.DefaultPatchMaker.Calculate(storedPdb, newPdb,
		patch.IgnorePDBSelector(),
		patch.IgnoreStatusFields(),
	)
	if err != nil {
		logger.Error(err, "Unable to patch redis PodDisruption with comparison object")
		return err
	}
	if !patchResult.IsEmpty() {
		logger.V(1).Info("Changes in PodDisruptionBudget Detected, Updating...",
			"patch", string(patchResult.Patch),
			"Current", string(patchResult.Current),
			"Original", string(patchResult.Original),
			"Modified", string(patchResult.Modified),
		)
		for key, value := range storedPdb.Annotations {
			if _, present := newPdb.Annotations[key]; !present {
				newPdb.Annotations[key] = value
			}
		}
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(newPdb); err != nil {
			logger.Error(err, "Unable to patch redis PodDisruptionBudget with comparison object")
			return err
		}
		return updatePodDisruptionBudget(namespace, newPdb, cl)
	}
	return nil
}

// createPodDisruptionBudget is a method to create PodDisruptionBudgets in Kubernetes
func createPodDisruptionBudget(namespace string, pdb *policyv1.PodDisruptionBudget, cl kubernetes.Interface) error {
	logger := pdbLogger(namespace, pdb.Name)
	_, err := cl.PolicyV1().PodDisruptionBudgets(namespace).Create(context.TODO(), pdb, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Redis PodDisruptionBudget creation failed")
		return err
	}
	logger.V(1).Info("Redis PodDisruptionBudget creation was successful")
	return nil
}

// updatePodDisruptionBudget is a method to update PodDisruptionBudgets in Kubernetes
func updatePodDisruptionBudget(namespace string, pdb *policyv1.PodDisruptionBudget, cl kubernetes.Interface) error {
	logger := pdbLogger(namespace, pdb.Name)
	_, err := cl.PolicyV1().PodDisruptionBudgets(namespace).Update(context.TODO(), pdb, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Redis PodDisruptionBudget update failed")
		return err
	}
	logger.V(1).Info("Redis PodDisruptionBudget update was successful", "PDB.Spec", pdb.Spec)
	return nil
}

// deletePodDisruptionBudget is a method to delete PodDisruptionBudgets in Kubernetes
func deletePodDisruptionBudget(namespace string, pdbName string, cl kubernetes.Interface) error {
	logger := pdbLogger(namespace, pdbName)
	err := cl.PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), pdbName, metav1.DeleteOptions{})
	if err != nil {
		logger.Error(err, "Redis PodDisruption deletion failed")
		return err
	}
	logger.V(1).Info("Redis PodDisruption delete was successful")
	return nil
}

// GetPodDisruptionBudget is a method to get PodDisruptionBudgets in Kubernetes
func GetPodDisruptionBudget(namespace string, pdb string, cl kubernetes.Interface) (*policyv1.PodDisruptionBudget, error) {
	logger := pdbLogger(namespace, pdb)
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("PodDisruptionBudget", "policy/v1"),
	}
	pdbInfo, err := cl.PolicyV1().PodDisruptionBudgets(namespace).Get(context.TODO(), pdb, getOpts)
	if err != nil {
		logger.V(1).Info("Redis PodDisruptionBudget get action failed")
		return nil, err
	}
	logger.V(1).Info("Redis PodDisruptionBudget get action was successful")
	return pdbInfo, err
}

// pdbLogger will generate logging interface for PodDisruptionBudgets
func pdbLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.PodDisruptionBudget.Namespace", namespace, "Request.PodDisruptionBudget.Name", name)
	return reqLogger
}
