package util

// MergeMap merges all the label maps received as argument into a single new label map.
func MergeMap(all ...map[string]string) map[string]string {
	res := map[string]string{}

	for _, labels := range all {
		for k, v := range labels {
			res[k] = v
		}
	}
	return res
}
