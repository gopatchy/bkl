//go:build !go1.21

package bkl

// Copied from go1.21 slices
func DeleteFunc[S ~[]E, E any](s S, del func(E) bool) S { //nolint:ireturn
	for i, v := range s {
		if del(v) {
			j := i

			for i++; i < len(s); i++ {
				v = s[i]
				if !del(v) {
					s[j] = v
					j++
				}
			}

			return s[:j]
		}
	}

	return s
}

// Copied from go1.21 slices
func Reverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
