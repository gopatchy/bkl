//go:build go1.20

package bkl

import "errors"

func ErrorsJoin(errs ...error) error {
	return errors.Join(errs...)
}
