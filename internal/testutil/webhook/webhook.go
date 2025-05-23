package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	controllerscheme "github.com/OT-CONTAINER-KIT/redis-operator/internal/controller/common/scheme"
	"github.com/stretchr/testify/require"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type ValidationWebhookTestCase struct {
	Name      string
	Operation admissionv1beta1.Operation
	Object    func(t *testing.T, uid string) []byte
	OldObject func(t *testing.T, uid string) []byte
	Check     func(t *testing.T, response *admissionv1beta1.AdmissionResponse)
}

func RunValidationWebhookTests(t *testing.T, gvk metav1.GroupVersionKind, validator admission.Validator, tests ...ValidationWebhookTestCase) {
	t.Helper()
	controllerscheme.SetupV1beta2Scheme()
	decoder := serializer.NewCodecFactory(clientgoscheme.Scheme).UniversalDeserializer()

	webhookHandler := admission.ValidatingWebhookFor(clientgoscheme.Scheme, validator)

	server := httptest.NewServer(webhookHandler)
	defer server.Close()

	client := server.Client()

	for _, tt := range tests {
		tc := tt
		t.Run(tc.Name, func(t *testing.T) {
			uid := tc.Name
			payload := &admissionv1beta1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{Kind: "AdmissionReview"},
				Request: &admissionv1beta1.AdmissionRequest{
					UID:       types.UID(uid),
					Kind:      gvk,
					Resource:  metav1.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: gvk.Kind},
					Operation: tc.Operation,
					Object:    runtime.RawExtension{Raw: tc.Object(t, uid)},
				},
			}

			if tc.Operation == admissionv1beta1.Update {
				payload.Request.OldObject = runtime.RawExtension{Raw: tc.OldObject(t, uid)}
			}

			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelFunc()

			request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, bytes.NewReader(payloadBytes))
			require.NoError(t, err)

			request.Header.Add("Content-Type", "application/json")
			resp, err := client.Do(request)
			require.NoError(t, err)
			defer func() {
				if resp.Body != nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}()

			response := decodeResponse(t, decoder, resp.Body)
			tc.Check(t, response)
		})
	}
}

func decodeResponse(t *testing.T, decoder runtime.Decoder, body io.Reader) *admissionv1beta1.AdmissionResponse {
	t.Helper()

	responseBytes, err := io.ReadAll(body)
	require.NoError(t, err, "Failed to read response body")

	response := &admissionv1beta1.AdmissionReview{}
	_, _, err = decoder.Decode(responseBytes, nil, response)
	require.NoError(t, err, "Failed to decode response")

	return response.Response
}

// ValidationWebhookSucceeded is a helper function to verify that the validation webhook accepted the request.
func ValidationWebhookSucceeded(t *testing.T, response *admissionv1beta1.AdmissionResponse) {
	t.Helper()
	require.True(t, response.Allowed, "Request denied: %s", response.Result.Reason)
}

// ValidationWebhookFailed is a helper function to verify that the validation webhook rejected the request.
func ValidationWebhookFailed(causeRegexes ...string) func(*testing.T, *admissionv1beta1.AdmissionResponse) {
	return func(t *testing.T, response *admissionv1beta1.AdmissionResponse) {
		t.Helper()
		require.False(t, response.Allowed)

		if len(causeRegexes) > 0 {
			require.NotNil(t, response.Result.Details, "Response must include failure details")
		}

		for _, cr := range causeRegexes {
			found := false
			t.Logf("Checking for existence of: %s", cr)
			for _, cause := range response.Result.Details.Causes {
				reason := fmt.Sprintf("%s: %s", cause.Field, cause.Message)
				t.Logf("Reason: %s", reason)
				match, err := regexp.MatchString(cr, reason)
				require.NoError(t, err, "Match '%s' returned error: %v", cr, err)
				if match {
					found = true
					break
				}
			}

			require.True(t, found, "[%s] is not present in cause list", cr)
		}
	}
}

func ValidationWebhookSucceededWithWarnings(warningsRegexes ...string) func(*testing.T, *admissionv1beta1.AdmissionResponse) {
	return func(t *testing.T, response *admissionv1beta1.AdmissionResponse) {
		t.Helper()
		require.True(t, response.Allowed, "Request denied: %s", response.Result.Reason)
		for _, wr := range warningsRegexes {
			found := false
			t.Logf("Checking for existence of: %s", wr)
			for _, warning := range response.Warnings {
				match, err := regexp.MatchString(wr, warning)
				require.NoError(t, err, "Match '%s' returned error: %v", wr, err)
				if match {
					found = true
					break
				}
			}
			require.True(t, found, "[%s] is not present in warning list", wr)
		}
	}
}
