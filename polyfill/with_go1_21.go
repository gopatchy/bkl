//go:build go1.21

package polyfill

import (
	"cmp"
	"maps"
	"slices"

	xmaps "golang.org/x/exp/maps"
)

func MapsClone[M ~map[K]V, K comparable, V any](m M) M { //nolint:ireturn
	return maps.Clone(m)
}

func MapsKeys[M ~map[K]V, K comparable, V any](m M) []K { //nolint:ireturn
	// Added to 1.21 maps then removed again
	return xmaps.Keys(m)
}

func SlicesClone[S ~[]E, E any](s S) S { //nolint:ireturn
	return slices.Clone(s)
}

func SlicesDeleteFunc[S ~[]E, E any](s S, del func(E) bool) S { //nolint:ireturn
	return slices.DeleteFunc(s, del)
}

func SlicesReverse[S ~[]E, E any](s S) {
	slices.Reverse(s)
}

func SlicesSort[S ~[]E, E cmp.Ordered](x S) {
	slices.Sort(x)
}
