//go:build !go1.20

package polyfill

func ErrorsJoin(errs ...error) error {
	return errs[0]
}
