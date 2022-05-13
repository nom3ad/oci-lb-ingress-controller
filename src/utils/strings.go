package utils

import (
	"reflect"

	"k8s.io/apimachinery/pkg/util/sets"
)

// IndexOfStr returns the first index of the target string t, or -1 if no match is found.
func IndexOfStr(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

// IncludesStr returns true if the target string t is in the slice.
func IncludesStr(vs []string, t string) bool {
	return IndexOfStr(vs, t) >= 0
}

// AnyStr returns true if one of the strings in the slice satisfies the predicate f.
func AnyStr(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if f(v) {
			return true
		}
	}
	return false
}

// AllStr returns true if all of the strings in the slice satisfy the predicate f.
func AllStr(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if !f(v) {
			return false
		}
	}
	return true
}

// FilterStr returns a new slice containing all strings in the slice that satisfy the predicate f.
func FilterStr(vs []string, f func(string) bool) []string {
	vsf := make([]string, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

// MapStr returns a new slice containing the results of applying the function f to each string in the original slice.
func MapStr(vs []string, f func(string) string) []string {
	vsm := make([]string, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}

func StringKeys(mapping interface{}) sets.String {
	keyset := sets.NewString()
	if mapping == nil {
		return keyset
	}
	val := reflect.ValueOf(mapping)
	for _, k := range val.MapKeys() {
		keyset.Insert(k.String())
	}
	return keyset
}

// StringKeyDiff  Keys(a) - Keys(b)
func StringKeyDiff(a, b interface{}) sets.String {
	keysetA := StringKeys(a)
	keysetB := StringKeys(b)
	return keysetA.Difference(keysetB)
}

// AllStr returns true if all of the strings in the slice satisfy the predicate f.
func StringListEqual(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if !f(v) {
			return false
		}
	}
	return true
}

func SafeSlice(slice string, start, end int) string {
	l := len(slice)
	if start >= l || end < start {
		return ""
	}
	if end > l {
		return slice[start:]
	}
	return slice[start:end]
}
