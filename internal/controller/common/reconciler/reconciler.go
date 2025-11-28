package reconciler

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Params struct {
	Context  context.Context
	Client   client.Client
	Owner    client.Object
	Expected client.Object

	Reconciled client.Object

	NeedsUpdate      func() bool
	UpdateReconciled func()
}

func Reconcile(params Params) error {
	if params.Reconciled == nil {
		return fmt.Errorf("params.Reconciled object must be provided")
	}
	if params.Owner != nil {
		if err := controllerutil.SetControllerReference(params.Owner, params.Expected, scheme.Scheme); err != nil {
			return err
		}
	}

	gvk, err := apiutil.GVKForObject(params.Expected, scheme.Scheme)
	if err != nil {
		return err
	}

	kind := gvk.Kind
	namespace := params.Expected.GetNamespace()
	name := params.Expected.GetName()
	log := log.FromContext(params.Context).WithValues("kind", kind, "namespace", namespace, "name", name)
	create := func() error {
		log.Info("Creating resource")
		expectedCopy := reflect.ValueOf(params.Expected.DeepCopyObject()).Elem()
		reflect.ValueOf(params.Reconciled).Elem().Set(expectedCopy)
		err = params.Client.Create(params.Context, params.Reconciled)
		if err != nil {
			return err
		}
		log.Info("Created resource successfully", "resourceVersion", params.Reconciled.GetResourceVersion())
		return nil
	}

	err = params.Client.Get(params.Context, types.NamespacedName{Name: name, Namespace: namespace}, params.Reconciled)
	if err != nil && apierrors.IsNotFound(err) {
		return create()
	} else if err != nil {
		return fmt.Errorf("failed to get %s %s/%s: %w", kind, namespace, name, err)
	}

	if params.NeedsUpdate() {
		log.Info("Updating resource")
		reconciledMeta, err := meta.Accessor(params.Reconciled)
		if err != nil {
			return err
		}

		resourceVersion := reconciledMeta.GetResourceVersion()
		params.UpdateReconciled()
		reconciledMeta.SetResourceVersion(resourceVersion)

		// We ensure the params.Owners is set as the controller owner reference before update.
		expectedMeta, err := meta.Accessor(params.Expected)
		if err != nil {
			return err
		}
		expectedOwners := expectedMeta.GetOwnerReferences()
		if len(expectedOwners) > 0 {
			reconciledOwners := reconciledMeta.GetOwnerReferences()
			if idx := indexOfControllerReference(reconciledOwners); idx == -1 {
				reconciledOwners = append(reconciledOwners, expectedOwners[0])
			} else {
				reconciledOwners[idx] = expectedOwners[0]
			}
			reconciledMeta.SetOwnerReferences(reconciledOwners)
		}

		err = params.Client.Update(params.Context, params.Reconciled)
		if err != nil {
			return err
		}
		log.Info("Updated resource successfully", "resourceVersion", params.Reconciled.GetResourceVersion())
	}
	return nil
}

func indexOfControllerReference(owners []metav1.OwnerReference) int {
	for index, r := range owners {
		if r.Controller != nil && *r.Controller {
			return index
		}
	}
	return -1
}
