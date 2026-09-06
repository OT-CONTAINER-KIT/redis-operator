package k8sutils

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

type contextPropagationKey struct{}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// responder lets a test override the response for a particular request. Returning
// nil falls through to the default success response.
type responder func(*http.Request) *http.Response

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newContextCheckingClient(t *testing.T, overrides ...responder) kubernetes.Interface {
	t.Helper()

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Context().Value(contextPropagationKey{}) != "reconcile" {
			return nil, errors.New("reconcile context was not propagated to the Kubernetes API request")
		}

		for _, override := range overrides {
			if resp := override(req); resp != nil {
				resp.Request = req
				return resp, nil
			}
		}

		body := `{}`
		switch {
		case strings.Contains(req.URL.Path, "/services"):
			body = `{"apiVersion":"v1","kind":"Service","metadata":{"name":"test","namespace":"test"}}`
		case strings.Contains(req.URL.Path, "/statefulsets"):
			body = `{"apiVersion":"apps/v1","kind":"StatefulSet","metadata":{"name":"test","namespace":"test"}}`
		case strings.Contains(req.URL.Path, "/poddisruptionbudgets"):
			body = `{"apiVersion":"policy/v1","kind":"PodDisruptionBudget","metadata":{"name":"test","namespace":"test"}}`
		case strings.Contains(req.URL.Path, "/persistentvolumeclaims") && req.Method == http.MethodGet:
			body = `{"apiVersion":"v1","kind":"PersistentVolumeClaimList","items":[{"metadata":{"name":"data-test-0","namespace":"test"},"spec":{"resources":{"requests":{"storage":"1Gi"}}}}]}`
		case strings.Contains(req.URL.Path, "/persistentvolumeclaims"):
			body = `{"apiVersion":"v1","kind":"PersistentVolumeClaim","metadata":{"name":"data-test-0","namespace":"test"},"spec":{"resources":{"requests":{"storage":"2Gi"}}}}`
		}

		resp := jsonResponse(http.StatusOK, body)
		resp.Request = req
		return resp, nil
	})

	client, err := kubernetes.NewForConfig(&rest.Config{
		Host:      "https://kubernetes.test",
		Transport: transport,
	})
	require.NoError(t, err)
	return client
}

func TestServiceHelpersPropagateContext(t *testing.T) {
	ctx := context.WithValue(t.Context(), contextPropagationKey{}, "reconcile")
	client := newContextCheckingClient(t)
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}}

	require.NoError(t, createService(ctx, client, "test", service))
	require.NoError(t, updateService(ctx, client, "test", service))
	_, err := getService(ctx, client, "test", "test")
	require.NoError(t, err)
}

func TestStatefulSetHelpersPropagateContext(t *testing.T) {
	ctx := context.WithValue(t.Context(), contextPropagationKey{}, "reconcile")
	client := newContextCheckingClient(t)
	statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}}

	require.NoError(t, createStatefulSet(ctx, client, "test", statefulSet))
	require.NoError(t, updateStatefulSet(ctx, client, "test", statefulSet, false, nil))
	_, err := GetStatefulSet(ctx, client, "test", "test")
	require.NoError(t, err)
}

// updateStatefulSet falls back to deleting the StatefulSet when the API server
// rejects the update as Invalid, and relies on the controller to recreate it.
// That DELETE is a request of its own, on a path the success case never reaches,
// so it needs its own coverage.
func TestStatefulSetRecreatePropagatesContext(t *testing.T) {
	ctx := context.WithValue(t.Context(), contextPropagationKey{}, "reconcile")

	var deleteRequests int
	client := newContextCheckingClient(t, func(req *http.Request) *http.Response {
		if !strings.Contains(req.URL.Path, "/statefulsets") {
			return nil
		}
		switch req.Method {
		case http.MethodPut:
			return jsonResponse(http.StatusUnprocessableEntity, `{"apiVersion":"v1","kind":"Status","status":"Failure","code":422,"reason":"Invalid","message":"StatefulSet.apps \"test\" is invalid","details":{"name":"test","group":"apps","kind":"StatefulSet","causes":[{"reason":"FieldValueForbidden","message":"updates to statefulset spec for fields other than 'replicas' are forbidden","field":"spec"}]}}`)
		case http.MethodDelete:
			deleteRequests++
			return jsonResponse(http.StatusOK, `{"apiVersion":"v1","kind":"Status","status":"Success"}`)
		}
		return nil
	})

	statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}}

	require.NoError(t, updateStatefulSet(ctx, client, "test", statefulSet, true, ptr.To(metav1.DeletePropagationForeground)))
	require.Equal(t, 1, deleteRequests, "the recreate branch should delete the StatefulSet once")
}

func TestPodDisruptionBudgetHelpersPropagateContext(t *testing.T) {
	ctx := context.WithValue(t.Context(), contextPropagationKey{}, "reconcile")
	client := newContextCheckingClient(t)
	pdb := &policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"}}

	require.NoError(t, createPodDisruptionBudget(ctx, "test", pdb, client))
	require.NoError(t, updatePodDisruptionBudget(ctx, "test", pdb, client))
	_, err := getPodDisruptionBudget(ctx, "test", "test", client)
	require.NoError(t, err)
	require.NoError(t, deletePodDisruptionBudget(ctx, "test", "test", client))
}

func TestPVCResizePropagatesContext(t *testing.T) {
	ctx := context.WithValue(t.Context(), contextPropagationKey{}, "reconcile")
	client := newContextCheckingClient(t)
	storedSize := resource.MustParse("1Gi")
	desiredSize := resource.MustParse("2Gi")
	stored := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   "test",
			Annotations: map[string]string{"storageCapacity": "1073741824"},
		},
		Spec: appsv1.StatefulSetSpec{VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: storedSize},
			}},
		}}},
	}
	desired := stored.DeepCopy()
	desired.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage] = desiredSize

	require.NoError(t, HandlePVCResizing(ctx, stored, desired, client))
}
