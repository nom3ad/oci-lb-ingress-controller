package utils

func ContainsMatching[T any](slice []T, matcher func(item T) bool) bool {
	for _, it := range slice {
		if matcher(it) {
			return true
		}
	}
	return false
}
