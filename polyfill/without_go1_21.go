//go:build !go1.21

package polyfill

import (
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// Copied from go1.21 maps
func MapsClone[M ~map[K]V, K comparable, V any](m M) M { //nolint:ireturn
	if m == nil {
		return nil
	}

	r := make(M, len(m))

	for k, v := range m {
		r[k] = v
	}

	return r
}

func MapsKeys[M ~map[K]V, K comparable, V any](m M) []K { //nolint:ireturn
	return maps.Keys(m)
}

func MapsValues[M ~map[K]V, K comparable, V any](m M) []V { //nolint:ireturn
	return maps.Values(m)
}

// Copied from go1.21 slices
func SlicesClone[S ~[]E, E any](s S) S { //nolint:ireturn
	if s == nil {
		return nil
	}

	return append(S([]E{}), s...)
}

// Copied from go1.21 slices
func SlicesDeleteFunc[S ~[]E, E any](s S, del func(E) bool) S { //nolint:ireturn
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
func SlicesReverse[S ~[]E, E any](s S) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func SlicesSort[E constraints.Ordered](x []E) {
	slices.Sort(x)
}
