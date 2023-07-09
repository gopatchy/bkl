//go:build go1.21

package bkl

import (
	"slices"
)

func slicesDeleteFunc[S ~[]E, E any](s S, del func(E) bool) S { //nolint:ireturn
	return slices.DeleteFunc(s, del)
}

func slicesReverse[S ~[]E, E any](s S) {
	slices.Reverse(s)
}
