//go:build go1.20

package bkl

import "errors"

func errorsJoin(errs ...error) error {
	return errors.Join(errs...)
}
