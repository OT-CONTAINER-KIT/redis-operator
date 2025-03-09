package util

// MergeMap merges all the label maps received as argument into a single new label map.
func MergeMap[K comparable, V any](all ...map[K]V) map[K]V {
	res := make(map[K]V)

	for _, m := range all {
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}
