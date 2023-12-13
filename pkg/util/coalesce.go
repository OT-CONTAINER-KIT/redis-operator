package util

func Coalesce[T comparable](val, defaultVal T) T {
	var t T
	if val == t {
		return defaultVal
	}
	return val
}
