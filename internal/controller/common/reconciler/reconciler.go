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
	Client   client.Client
	Owner    client.Object
	Expected client.Object

	NeedUpdate func(existed client.Object) bool
	Update     func(existed client.Object)
}

func Reconcile(ctx context.Context, params Params) (client.Object, error) {
	if params.Owner != nil {
		if err := controllerutil.SetControllerReference(params.Owner, params.Expected, scheme.Scheme); err != nil {
			return nil, err
		}
	}

	gvk, err := apiutil.GVKForObject(params.Expected, scheme.Scheme)
	if err != nil {
		return nil, err
	}

	kind := gvk.Kind
	namespace := params.Expected.GetNamespace()
	name := params.Expected.GetName()
	// Create a new instance of the same type as Expected
	existed := reflect.New(reflect.TypeOf(params.Expected).Elem()).Interface().(client.Object)
	log := log.FromContext(ctx).WithValues("kind", kind, "namespace", namespace, "name", name)
	create := func() error {
		log.Info("Creating resource")
		expectedCopy := reflect.ValueOf(params.Expected.DeepCopyObject()).Elem()
		reflect.ValueOf(existed).Elem().Set(expectedCopy)
		err = params.Client.Create(ctx, existed)
		if err != nil {
			return err
		}
		log.Info("Created resource successfully", "resourceVersion", existed.GetResourceVersion())
		return nil
	}

	err = params.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existed)
	if err != nil && apierrors.IsNotFound(err) {
		return existed, create()
	} else if err != nil {
		return nil, fmt.Errorf("failed to get %s %s/%s: %w", kind, namespace, name, err)
	}

	if params.NeedUpdate(existed) {
		log.Info("Updating resource")
		reconciledMeta, err := meta.Accessor(existed)
		if err != nil {
			return nil, err
		}

		resourceVersion := reconciledMeta.GetResourceVersion()
		params.Update(existed)
		reconciledMeta.SetResourceVersion(resourceVersion)

		// We ensure the params.Owners is set as the controller owner reference before update.
		expectedMeta, err := meta.Accessor(params.Expected)
		if err != nil {
			return nil, err
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

		err = params.Client.Update(ctx, existed)
		if err != nil {
			return nil, err
		}
		log.Info("Updated resource successfully", "resourceVersion", existed.GetResourceVersion())
	}
	return existed, nil
}

func indexOfControllerReference(owners []metav1.OwnerReference) int {
	for index, r := range owners {
		if r.Controller != nil && *r.Controller {
			return index
		}
	}
	return -1
}
