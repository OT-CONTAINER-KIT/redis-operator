package client

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type MockClient struct {
	GetFn                func(ctx context.Context, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error
	ListFn               func(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error
	CreateFn             func(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.CreateOption) error
	DeleteFn             func(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.DeleteOption) error
	UpdateFn             func(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.UpdateOption) error
	PatchFn              func(ctx context.Context, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error
	DeleteAllofFn        func(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.DeleteAllOfOption) error
	IsObjectNamespacedFn func(obj runtime.Object) (bool, error)
	ctrlclient.StatusClient
	ctrlclient.SubResourceClientConstructor
}

func (m *MockClient) Scheme() *runtime.Scheme {
	return nil
}

func (m *MockClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (m *MockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (m *MockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return m.IsObjectNamespacedFn(obj)
}

func (m *MockClient) Get(ctx context.Context, key ctrlclient.ObjectKey, obj ctrlclient.Object, opts ...ctrlclient.GetOption) error {
	return m.GetFn(ctx, key, obj, opts...)
}

func (m *MockClient) List(ctx context.Context, list ctrlclient.ObjectList, opts ...ctrlclient.ListOption) error {
	return m.ListFn(ctx, list, opts...)
}

func (m *MockClient) Create(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.CreateOption) error {
	return m.CreateFn(ctx, obj, opts...)
}

func (m *MockClient) Delete(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.DeleteOption) error {
	return m.DeleteFn(ctx, obj, opts...)
}

func (m *MockClient) Update(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.UpdateOption) error {
	return m.UpdateFn(ctx, obj, opts...)
}

func (m *MockClient) Patch(ctx context.Context, obj ctrlclient.Object, patch ctrlclient.Patch, opts ...ctrlclient.PatchOption) error {
	return m.PatchFn(ctx, obj, patch, opts...)
}

func (m *MockClient) DeleteAllOf(ctx context.Context, obj ctrlclient.Object, opts ...ctrlclient.DeleteAllOfOption) error {
	return m.DeleteAllofFn(ctx, obj, opts...)
}
