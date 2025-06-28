package utils

import (
	"fmt"

	"github.com/gopatchy/bkl/pkg/errors"
)

func PopListString(l []any, v string) (bool, []any) {
	found := false

	l, _ = FilterList(l, func(x any) ([]any, error) {
		s, ok := x.(string)
		if !ok {
			return []any{x}, nil
		}

		if s == v {
			found = true
			return nil, nil
		}

		return []any{x}, nil
	})

	return found, l
}

func PopListMapValue(l []any, k string) (any, []any, error) {
	var ret any

	l, err := FilterList(l, func(x any) ([]any, error) {
		xMap, ok := x.(map[string]any)
		if !ok {
			return []any{x}, nil
		}

		if len(xMap) != 1 {
			return []any{x}, nil
		}

		found, val, xMap := PopMapValue(xMap, k)
		if found {
			if ret != nil {
				return nil, fmt.Errorf("%#v: %w", l, errors.ErrExtraKeys)
			}

			ret = val

			return nil, nil
		}

		return []any{xMap}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return ret, l, nil
}

func PopListMapBoolValue(l []any, k string, v bool) (bool, []any, error) {
	if !HasListMapBoolValue(l, k, v) {
		return false, l, nil
	}

	l, err := FilterList(l, func(x any) ([]any, error) {
		xMap, ok := x.(map[string]any)
		if !ok {
			return []any{x}, nil
		}

		// Directive isolation: maps with multiple keys aren't pure directives
		if len(xMap) != 1 {
			return []any{x}, nil
		}

		found, xMap := PopMapBoolValue(xMap, k, v)
		if found {
			if len(xMap) > 0 {
				return nil, fmt.Errorf("%#v: %w", xMap, errors.ErrExtraKeys)
			}

			return nil, nil
		}

		return []any{x}, nil
	})
	if err != nil {
		return false, nil, err
	}

	return true, l, nil
}
