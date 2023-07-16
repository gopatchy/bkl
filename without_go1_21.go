//go:build !go1.21

package bkl

// Copied from go1.21 maps
func mapsClone[M ~map[K]V, K comparable, V any](m M) M { //nolint:ireturn
	if m == nil {
		return nil
	}

	r := make(M, len(m))

	for k, v := range m {
		r[k] = v
	}

	return r
}

// Copied from go1.21 slices
func slicesReverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
