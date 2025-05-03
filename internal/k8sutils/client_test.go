package k8sutils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

func mockK8sConfigProvider() (*rest.Config, error) {
	return &rest.Config{}, nil
}

func mockInvalidK8sConfigProvider() (*rest.Config, error) {
	return nil, errors.New("invalid configuration")
}

func TestGenerateK8sClient(t *testing.T) {
	tests := []struct {
		name           string
		configProvider func() (*rest.Config, error)
		wantErr        bool
	}{
		{
			name:           "valid config",
			configProvider: mockK8sConfigProvider,
			wantErr:        false,
		},
		{
			name:           "invalid config",
			configProvider: mockInvalidK8sConfigProvider,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := GenerateK8sClient(tt.configProvider)
			if tt.wantErr {
				assert.Error(t, err, "GenerateK8sClient() should return an error for invalid config")
			} else {
				assert.NoError(t, err, "GenerateK8sClient() should not return an error for valid config")
				assert.NotNil(t, client, "expected a non-nil Kubernetes client")
			}
		})
	}
}

func TestGenerateK8sDynamicClient(t *testing.T) {
	tests := []struct {
		name           string
		configProvider func() (*rest.Config, error)
		wantErr        bool
	}{
		{
			name:           "valid config",
			configProvider: mockK8sConfigProvider,
			wantErr:        false,
		},
		{
			name:           "invalid config",
			configProvider: mockInvalidK8sConfigProvider,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := GenerateK8sDynamicClient(tt.configProvider)
			if tt.wantErr {
				assert.Error(t, err, "GenerateK8sDynamicClient() should return an error for invalid config")
			} else {
				assert.NoError(t, err, "GenerateK8sDynamicClient() should not return an error for valid config")
				assert.NotNil(t, client, "expected a non-nil Kubernetes client")
			}
		})
	}
}
