package util

import "os"

func Coalesce[T comparable](val, defaultVal T) T {
	var t T
	if val == t {
		return defaultVal
	}
	return val
}

// CoalesceEnv returns the value of the environment variable or the default value.
// The second return value indicates whether the environment variable was set.
func CoalesceEnv(key, defaultVal string) (string, bool) {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal, false
	}
	return val, true
}

// CoalesceEnv1 returns the value of the environment variable or the default value.
// Unlike CoalesceEnv, it only returns the value without a boolean indicator.
func CoalesceEnv1(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}
