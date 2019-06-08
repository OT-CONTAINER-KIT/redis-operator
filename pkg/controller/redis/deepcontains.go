package redis

import (
	"reflect"
	"unsafe"
)

// During deepValueContains, must keep track of checks that are
// in progress. The comparison algorithm assumes that all
// checks in progress are true when it reencounters them.
// Visited comparisons are stored in a map indexed by visit.
type visit struct {
	a1  unsafe.Pointer
	a2  unsafe.Pointer
	typ reflect.Type
}

// deepContains reports whether y is a ``subset'' of x defined as follows.
// It is a relaxation of the reflect.DeepEqual aimed to compare deeply nested values.
// Value x deeply contains value y if one of the following cases applies.
// Values of distinct types never contain each other.
//
// Array x deeply contains array y when the elements of array y deeply contain corresponding elements of array x.
//
// Struct value x deeply contains struct value y if the exported values of y deeply contain corresponding exported fields of x.
// Unexported fields are ignored.
//
// Func values deeply contain each other if both are nil.
//
// Interface value x deeply contains interface value y if y holds values deeply equal to the concrete values of x, nil or zero values.
//
// Map value x deeply contains map value y when one of the following is true:
// - map value y is nil or zero value,
// - length of y is less or equal to the length of x, and they are the same map object or the keys of y match
// corresponding keys of x (matched using Go equality) and map to deeply contained values.
//
// Pointer values are deeply contained if they are equal using Go's == operator
// or if they point to deeply contained values.
//
// Slice values are deeply contained when one of the following is true:
// - y is nil or zero value,
// - length of y is less or equal to the length of x, and either they point to the same initial entry of the same underlying array
// (that is, &x[0] == &y[0]) or their corresponding elements (up to length of y) are deeply contained.
// Note that a non-nil empty slice and a nil slice (for example, []byte{} and []byte(nil)) are deeply contained.
//
// Other values - numbers, bools, strings, and channels - are deeply contained
// if y is zero value or they are equal using Go's == operator.
//
// In general deepContains is a relaxation of the reflect.DeepEqual aimed to compare deeply nested values.
// However, this idea is impossible to implement without some inconsistency.
// Specifically, it is possible for a value to be unequal to itself,
// either because it is of func type (uncomparable in general)
// or because it is a floating-point NaN value (not equal to itself in floating-point comparison),
// or because it is an array, struct, or interface containing
// such a value.
//
// Another important caveat is zero and nil values.
// Zero values are indistinguishable from those declared explicitly and will be treated as deeply contained
// when compared against non-zero values.
// Deeply nested values y of basic numeric, string and bool types are always compared against their zero values
// and they are equal to zero values the are treated as "contained" for the corresponding values of x.
//
// On the other hand, pointer values are always equal to themselves,
// even if they point at or contain such problematic values,
// because they compare equal using Go's == operator, and that
// is a sufficient condition to be deeply contained, regardless of content.
// deepContains has been defined so that the same short-cut applies
// to slices and maps: if x and y are the same slice or the same map,
// they are deeply contained regardless of content.
//
// As deepContains traverses the data values it may find a cycle. The
// second and subsequent times that deepContains compares two pointer
// values that have been compared before, it treats the values as
// equal rather than examining the values to which they point.
// This ensures that deepContains terminates.
func deepContains(x, y interface{}) bool {
	if x == nil || y == nil {
		return x == y
	}
	v1 := reflect.ValueOf(x)
	v2 := reflect.ValueOf(y)
	if v1.Type() != v2.Type() {
		return false
	}
	return deepValueContains(v1, v2, make(map[visit]bool), 0)
}

// Tests for deep equality using reflected types. The map argument tracks
// comparisons that have already been seen, which allows short circuiting on
// recursive types.
func deepValueContains(v1, v2 reflect.Value, visited map[visit]bool, depth int) bool {
	// ignore zero values
	if !v1.IsValid() || !v2.IsValid() {
		return true
	}
	if v1.Type() != v2.Type() {
		return false
	}

	// if depth > 10 { panic("deepValueContains") }	// for debugging

	// We want to avoid putting more in the visited map than we need to.
	// For any possible reference cycle that might be encountered,
	// hard(t) needs to return true for at least one of the types in the cycle.
	hard := func(k reflect.Kind) bool {
		switch k {
		case reflect.Map, reflect.Slice, reflect.Ptr, reflect.Interface:
			return true
		}
		return false
	}

	if v1.CanAddr() && v2.CanAddr() && hard(v1.Kind()) {
		addr1 := unsafe.Pointer(v1.UnsafeAddr())
		addr2 := unsafe.Pointer(v2.UnsafeAddr())
		if uintptr(addr1) > uintptr(addr2) {
			// Canonicalize order to reduce number of entries in visited.
			// Assumes non-moving garbage collector.
			addr1, addr2 = addr2, addr1
		}

		// Short circuit if references are already seen.
		typ := v1.Type()
		v := visit{addr1, addr2, typ}
		if visited[v] {
			return true
		}

		// Remember for later.
		visited[v] = true
	}

	switch v1.Kind() {
	case reflect.Array:
		for i := 0; i < v2.Len(); i++ {
			if !deepValueContains(v1.Index(i), v2.Index(i), visited, depth+1) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if v2.IsNil() {
			return true
		}
		if v1.Len() < v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for i := 0; i < v2.Len(); i++ {
			if !deepValueContains(v1.Index(i), v2.Index(i), visited, depth+1) {
				return false
			}
		}
		return true
	case reflect.Interface:
		return v2.IsNil() || deepValueContains(v1.Elem(), v2.Elem(), visited, depth+1)
	case reflect.Ptr:
		return v1.Pointer() == v2.Pointer() || deepValueContains(v1.Elem(), v2.Elem(), visited, depth+1)
	case reflect.Struct:
		for i, n := 0, v2.NumField(); i < n; i++ {
			if !deepValueContains(v1.Field(i), v2.Field(i), visited, depth+1) {
				return false
			}
		}
		return true
	case reflect.Map:
		if v2.IsNil() {
			return true
		}
		if v1.Len() < v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for _, k := range v2.MapKeys() {
			val1 := v1.MapIndex(k)
			val2 := v2.MapIndex(k)
			if !val1.IsValid() || !val2.IsValid() || !deepValueContains(val1, val2, visited, depth+1) {
				return false
			}
		}
		return true
	case reflect.Func:
		return v1.IsNil() && v2.IsNil()
	default:
		return isEmptyValue(v2) || compareOrTrue(v1, v2)
	}
}

// compareOrTrue tries to compare two values normally.
// It ignores panic if any of the Values were obtained by accessing unexported struct fields
func compareOrTrue(v1, v2 reflect.Value) (res bool) {
	defer func() {
		if r := recover(); r != nil {
			res = true
		}
	}()
	return v1.Interface() == v2.Interface()
}

// From src/pkg/encoding/json/encode.go.
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.String:
		return v.Len() == 0
	}
	return false
}
