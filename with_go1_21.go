//go:build go1.21

package bkl

import (
	"cmp"
	"maps"
	"slices"
)

func mapsClone[M ~map[K]V, K comparable, V any](m M) M { //nolint:ireturn
	return maps.Clone(m)
}

func mapsKeys[M ~map[K]V, K comparable, V any](m M) []K { //nolint:ireturn
	return maps.Keys(m)
}

func slicesReverse[S ~[]E, E any](s S) {
	slices.Reverse(s)
}

func slicesSort[S ~[]E, E cmp.Ordered](x S) {
	slices.Sort(s)
}
