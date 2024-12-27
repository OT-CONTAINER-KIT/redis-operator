package webhook

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestPodAntiAffinityMutate_Handle(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	decoder := admission.NewDecoder(scheme)
	logger := zap.New(zap.WriteTo(nil))

	mutator := NewPodAffiniytMutate(fakeClient, decoder, logger)

	tests := []struct {
		name            string
		pod             *corev1.Pod
		expectedPatches []jsonpatch.JsonPatchOperation
	}{
		{
			name: "Should mutate pod with anti-affinity",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-leader-0",
					Namespace: "default",
					Annotations: map[string]string{
						annotationKeyEnablePodAntiAffinity: "true",
						podAnnotationsRedisClusterApp:      "db-01",
					},
					Labels: map[string]string{
						podLabelsRedisType: "redis-cluster",
					},
				},
				Spec: corev1.PodSpec{},
			},
			expectedPatches: []jsonpatch.JsonPatchOperation{
				{
					Operation: "add",
					Path:      "/spec/affinity",
					Value: map[string]interface{}{
						"podAntiAffinity": map[string]interface{}{
							"requiredDuringSchedulingIgnoredDuringExecution": []interface{}{
								map[string]interface{}{
									"labelSelector": map[string]interface{}{
										"matchExpressions": []interface{}{
											map[string]interface{}{
												"key":      "statefulset.kubernetes.io/pod-name",
												"operator": "In",
												"values":   []interface{}{"redis-follower-0"},
											},
										},
									},
									"topologyKey": "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Should not mutate pod without proper annotations",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "redis-follower-0",
					Namespace: "default",
					Annotations: map[string]string{
						podAnnotationsRedisClusterApp: "db-01",
					},
					Labels: map[string]string{
						podLabelsRedisType: "redis-cluster",
					},
				},
				Spec: corev1.PodSpec{},
			},
			expectedPatches: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podBytes, err := json.Marshal(tt.pod)
			assert.NoError(t, err)

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Namespace: "default",
					Object: runtime.RawExtension{
						Raw: podBytes,
					},
				},
			}

			resp := mutator.Handle(context.Background(), req)
			assert.True(t, resp.Allowed)
			assert.Equal(t, tt.expectedPatches, resp.Patches)
		})
	}
}
