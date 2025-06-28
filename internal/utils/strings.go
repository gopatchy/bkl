package utils

import (
	"fmt"

	"github.com/gopatchy/bkl/pkg/errors"
)

func ToStringList(l []any) ([]string, error) {
	ret := []string{}

	for _, v := range l {
		switch v2 := v.(type) {
		case string:
			ret = append(ret, v2)
		default:
			return nil, fmt.Errorf("%T: %w", v, errors.ErrInvalidType)
		}
	}

	return ret, nil
}

func ToStringListPermissive(v any) ([]string, error) {
	v2, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%T: %w", v, errors.ErrInvalidType)
	}

	ret := []string{}
	for _, v3 := range v2 {
		ret = append(ret, fmt.Sprintf("%v", v3))
	}

	return ret, nil
}
