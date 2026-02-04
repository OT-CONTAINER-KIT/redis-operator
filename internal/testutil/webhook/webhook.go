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
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	cradmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ValidationWebhookTestCase describes a single admission operation to test.
type ValidationWebhookTestCase struct {
	Name      string
	Operation admissionv1.Operation
	Object    func(t *testing.T, uid string) []byte
	OldObject func(t *testing.T, uid string) []byte // required for Update
	Check     func(t *testing.T, response *admissionv1.AdmissionResponse)
}

// RunValidationWebhookTests spins up an httptest server running a validating admission webhook
// produced by controller-runtime and runs the provided test cases against it.
//
// NOTE: obj is required so controller-runtime knows what concrete Go type to decode into.
func RunValidationWebhookTests(
	t *testing.T,
	gvk metav1.GroupVersionKind,
	obj runtime.Object,
	validator cradmission.CustomValidator,
	tests ...ValidationWebhookTestCase,
) {
	t.Helper()

	controllerscheme.SetupV1beta2Scheme()

	// Build a validating webhook for the provided type + validator.
	hook := cradmission.WithCustomValidator(clientgoscheme.Scheme, obj, validator)

	// StandaloneWebhook wires up things webhook.Server would normally populate (good for tests).
	handler, err := cradmission.StandaloneWebhook(hook, cradmission.StandaloneOptions{})
	require.NoError(t, err)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := server.Client()

	for _, tt := range tests {
		tc := tt
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()

			uid := tc.Name
			payload := &admissionv1.AdmissionReview{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "admission.k8s.io/v1",
					Kind:       "AdmissionReview",
				},
				Request: &admissionv1.AdmissionRequest{
					UID:       types.UID(uid),
					Kind:      gvk,
					Resource:  metav1.GroupVersionResource{Group: gvk.Group, Version: gvk.Version, Resource: gvk.Kind},
					Operation: tc.Operation,
					Object:    runtime.RawExtension{Raw: tc.Object(t, uid)},
				},
			}

			if tc.Operation == admissionv1.Update {
				require.NotNil(t, tc.OldObject, "OldObject must be provided for Update operations")
				payload.Request.OldObject = runtime.RawExtension{Raw: tc.OldObject(t, uid)}
			}

			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, bytes.NewReader(payloadBytes))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			require.NoError(t, err)
			t.Cleanup(func() {
				if resp.Body != nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
				}
			})

			response := decodeResponse(t, resp.Body)
			tc.Check(t, response)
		})
	}
}

func decodeResponse(t *testing.T, body io.Reader) *admissionv1.AdmissionResponse {
	t.Helper()

	var review admissionv1.AdmissionReview
	err := json.NewDecoder(body).Decode(&review)
	require.NoError(t, err, "Failed to decode AdmissionReview response")

	require.NotNil(t, review.Response, "AdmissionReview response is nil")
	return review.Response
}

// ValidationWebhookSucceeded verifies that the webhook accepted the request.
func ValidationWebhookSucceeded(t *testing.T, response *admissionv1.AdmissionResponse) {
	t.Helper()
	require.True(t, response.Allowed, "Request denied: %s", statusMessage(response))
}

// ValidationWebhookFailed verifies that the webhook rejected the request and optionally matches causes.
func ValidationWebhookFailed(causeRegexes ...string) func(*testing.T, *admissionv1.AdmissionResponse) {
	return func(t *testing.T, response *admissionv1.AdmissionResponse) {
		t.Helper()
		require.False(t, response.Allowed, "Expected request to be denied")

		if len(causeRegexes) > 0 {
			require.NotNil(t, response.Result, "Response must include status")
			require.NotNil(t, response.Result.Details, "Response must include failure details")
		}

		for _, cr := range causeRegexes {
			found := false
			t.Logf("Checking for existence of: %s", cr)

			for _, cause := range response.Result.Details.Causes {
				reason := fmt.Sprintf("%s: %s", cause.Field, cause.Message)
				t.Logf("Cause: %s", reason)

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

func ValidationWebhookSucceededWithWarnings(warningsRegexes ...string) func(*testing.T, *admissionv1.AdmissionResponse) {
	return func(t *testing.T, response *admissionv1.AdmissionResponse) {
		t.Helper()
		require.True(t, response.Allowed, "Request denied: %s", statusMessage(response))

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

func statusMessage(resp *admissionv1.AdmissionResponse) string {
	if resp == nil || resp.Result == nil {
		return ""
	}
	if resp.Result.Message != "" {
		return resp.Result.Message
	}
	// Fallback (older API servers / odd cases)
	return string(resp.Result.Reason)
}
