package testutil

import "time"

const (
	// Common test namespace
	DefaultTestNamespace = "default"

	// Common test timeouts
	DefaultTimeout  = time.Second * 10
	DefaultInterval = time.Millisecond * 250

	// Common test image
	DefaultRedisImage = "quay.io/opstree/redis:v7.0.12"
)
