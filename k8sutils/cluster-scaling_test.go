package k8sutils

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-redis/redismock/v9"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func Test_verifyLeaderPodInfo(t *testing.T) {
	logger := logr.Discard()

	tests := []struct {
		name         string
		section      string
		response     string
		err          error
		expectedBool bool
	}{
		{
			name:         "is master",
			section:      "replication",
			response:     "role:master\r\n",
			expectedBool: true,
		},
		{
			name:         "is replica",
			section:      "replication",
			response:     "role:slave\r\n",
			expectedBool: false,
		},
		{
			name:         "redis info error",
			section:      "replication",
			err:          redis.ErrClosed,
			expectedBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client, mock := redismock.NewClientMock()

			if tt.err != nil {
				mock.ExpectInfo(tt.section).SetErr(tt.err)
			} else {
				mock.ExpectInfo(tt.section).SetVal(tt.response)
			}

			result := verifyLeaderPodInfo(ctx, client, logger, "test-pod")

			assert.Equal(t, tt.expectedBool, result, "Test case: "+tt.name)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unmet expectations: %s", err)
			}
		})
	}
}
