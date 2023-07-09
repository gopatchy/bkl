//go:build !go1.20

package bkl

func ErrorsJoin(errs ...error) error {
	return errs[0]
}
