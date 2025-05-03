package util

import "os"

func Coalesce[T comparable](val, defaultVal T) T {
	var t T
	if val == t {
		return defaultVal
	}
	return val
}

func CoalesceEnv(key, defaultVal string) (string, bool) {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal, false
	}
	return val, true
}
