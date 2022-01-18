package k8sutils

import (
	"context"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/go-logr/logr"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	redisv1beta1 "redis-operator/api/v1beta1"
)

// CreateRedisLeaderPodDisruptionBudget check and create a PodDisruptionBudget for Leaders
func ReconcileRedisPodDisruptionBudget(cr *redisv1beta1.RedisCluster, role string) error {
	pdbName := cr.ObjectMeta.Name + "-" + role
	logger := pdbLogger(cr.Namespace, pdbName)
	if cr.Spec.RedisLeader.PodDisruptionBudget != nil && cr.Spec.RedisLeader.PodDisruptionBudget.Enabled {
		labels := getRedisLabels(cr.ObjectMeta.Name, "cluster", role, cr.ObjectMeta.GetLabels())
		annotations := generateStatefulSetsAnots(cr.ObjectMeta)
		pdbMeta := generateObjectMetaInformation(pdbName, cr.Namespace, labels, annotations)
		pdbDef := generatePodDisruptionBudgetDef(cr, role, pdbMeta, cr.Spec.RedisLeader.PodDisruptionBudget)
		return CreateOrUpdatePodDisruptionBudget(pdbDef)
	} else {
		// Check if one exists, and delete it.
		_, err := GetPodDisruptionBudget(cr.Namespace, pdbName)
		if err == nil {
			return deletePodDisruptionBudget(cr.Namespace, pdbName)
		} else if err != nil && errors.IsNotFound(err) {
			logger.Info("Reconciliation Successful, no PodDisruptionBudget Found.")
			// Its ok if its not found, as we're deleting anyway
			return nil
		}
		return err
	}
}

// generatePodDisruptionBudgetDef will create a PodDisruptionBudget definition
func generatePodDisruptionBudgetDef(cr *redisv1beta1.RedisCluster, role string, pdbMeta metav1.ObjectMeta, pdbParams *redisv1beta1.RedisPodDisruptionBudget) *policyv1.PodDisruptionBudget {
	lblSelector := LabelSelectors(map[string]string{
		"app":  cr.ObjectMeta.Name,
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
		pdbTemplate.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: int32(*pdbParams.MinAvailable)}
	}
	if pdbParams.MaxUnavailable != nil {
		pdbTemplate.Spec.MaxUnavailable = &intstr.IntOrString{Type: intstr.Int, IntVal: int32(*pdbParams.MaxUnavailable)}
	}
	// If we don't have a value for either, assume quorum: (N/2)+1
	if pdbTemplate.Spec.MaxUnavailable == nil && pdbTemplate.Spec.MinAvailable == nil {
		pdbTemplate.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: int32((*cr.Spec.Size / 2) + 1)}
	}
	AddOwnerRefToObject(pdbTemplate, redisClusterAsOwner(cr))
	return pdbTemplate
}

// CreateOrUpdateService method will create or update Redis service
func CreateOrUpdatePodDisruptionBudget(pdbDef *policyv1.PodDisruptionBudget) error {
	logger := pdbLogger(pdbDef.Namespace, pdbDef.Name)
	storedPDB, err := GetPodDisruptionBudget(pdbDef.Namespace, pdbDef.Name)
	if err != nil {
		if err := patch.DefaultAnnotator.SetLastAppliedAnnotation(pdbDef); err != nil {
			logger.Error(err, "Unable to patch redis PodDisruptionBudget with comparison object")
			return err
		}
		if errors.IsNotFound(err) {
			return createPodDisruptionBudget(pdbDef.Namespace, pdbDef)
		}
		return err
	}
	return patchPodDisruptionBudget(storedPDB, pdbDef, pdbDef.Namespace)
}

// patchPodDisruptionBudget will patch Redis Kubernetes PodDisruptionBudgets
func patchPodDisruptionBudget(storedPdb *policyv1.PodDisruptionBudget, newPdb *policyv1.PodDisruptionBudget, namespace string) error {
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
		logger.Info("Changes in PodDisruptionBudget Detected, Updating...",
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
		return updatePodDisruptionBudget(namespace, newPdb)
	}
	return nil
}

// createPodDisruptionBudget is a method to create PodDisruptionBudgets in Kubernetes
func createPodDisruptionBudget(namespace string, pdb *policyv1.PodDisruptionBudget) error {
	logger := pdbLogger(namespace, pdb.Name)
	_, err := generateK8sClient().PolicyV1().PodDisruptionBudgets(namespace).Create(context.TODO(), pdb, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "Redis PodDisruptionBudget creation failed")
		return err
	}
	logger.Info("Redis PodDisruptionBudget creation was successful")
	return nil
}

// updatePodDisruptionBudget is a method to update PodDisruptionBudgets in Kubernetes
func updatePodDisruptionBudget(namespace string, pdb *policyv1.PodDisruptionBudget) error {
	logger := pdbLogger(namespace, pdb.Name)
	_, err := generateK8sClient().PolicyV1().PodDisruptionBudgets(namespace).Update(context.TODO(), pdb, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "Redis PodDisruptionBudget update failed")
		return err
	}
	logger.Info("Redis PodDisruptionBudget update was successful", "PDB.Spec", pdb.Spec)
	return nil
}

// deletePodDisruptionBudget is a method to delete PodDisruptionBudgets in Kubernetes
func deletePodDisruptionBudget(namespace string, pdbName string) error {
	logger := pdbLogger(namespace, pdbName)
	err := generateK8sClient().PolicyV1().PodDisruptionBudgets(namespace).Delete(context.TODO(), pdbName, metav1.DeleteOptions{})
	if err != nil {
		logger.Error(err, "Redis PodDisruption deletion failed")
		return err
	}
	logger.Info("Redis PodDisruption delete was successful")
	return nil
}

// GetPodDisruptionBudget is a method to get PodDisruptionBudgets in Kubernetes
func GetPodDisruptionBudget(namespace string, pdb string) (*policyv1.PodDisruptionBudget, error) {
	logger := pdbLogger(namespace, pdb)
	getOpts := metav1.GetOptions{
		TypeMeta: generateMetaInformation("PodDisruptionBudget", "policy/v1"),
	}
	pdbInfo, err := generateK8sClient().PolicyV1().PodDisruptionBudgets(namespace).Get(context.TODO(), pdb, getOpts)
	if err != nil {
		logger.Info("Redis PodDisruptionBudget get action failed")
		return nil, err
	}
	logger.Info("Redis PodDisruptionBudget get action was successful")
	return pdbInfo, err
}

// pdbLogger will generate logging interface for PodDisruptionBudgets
func pdbLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.PodDisruptionBudget.Namespace", namespace, "Request.PodDisruptionBudget.Name", name)
	return reqLogger
}
