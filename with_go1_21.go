//go:build go1.21

package bkl

import (
	"maps"
	"slices"
)

func mapsClone[M ~map[K]V, K comparable, V any](m M) M { //nolint:ireturn
	return maps.Clone(m)
}

func slicesDeleteFunc[S ~[]E, E any](s S, del func(E) bool) S { //nolint:ireturn
	return slices.DeleteFunc(s, del)
}

func slicesReverse[S ~[]E, E any](s S) {
	slices.Reverse(s)
}
