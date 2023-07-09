//go:build !go1.20

package bkl

func errorsJoin(errs ...error) error {
	return errs[0]
}
