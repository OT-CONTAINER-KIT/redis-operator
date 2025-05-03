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

// CopyMap creates a shallow copy of the given map.
// It returns a new map containing all key-value pairs from the source map.
// Note: This is a shallow copy; if the values are reference types,
// the copied map will reference the same underlying objects as the source.
func CopyMap[K comparable, V any](src map[K]V) map[K]V {
	return MergeMap(src)
}

// MergePreservingExistingKeys merges source into destination while skipping any keys that exist in the destination.
func MergePreservingExistingKeys[K comparable, V any](dest, src map[K]V) map[K]V {
	if dest == nil {
		if src == nil {
			return nil
		}
		dest = make(map[K]V, len(src))
	}

	for k, v := range src {
		if _, exists := dest[k]; !exists {
			dest[k] = v
		}
	}

	return dest
}
