package bkl

import (
	"fmt"
)

func normalize(obj any) (any, error) {
	switch objType := obj.(type) {
	case map[any]any:
		return nil, fmt.Errorf("numeric keys not supported (%w)", ErrInvalidType)

	case []map[string]any:
		ret := []any{}

		for _, v := range objType {
			v2, err := normalize(v)
			if err != nil {
				return nil, err
			}

			ret = append(ret, v2)
		}

		return ret, nil

	case map[string]any:
		ret := map[string]any{}

		for k, v := range objType {
			v2, err := normalize(v)
			if err != nil {
				return nil, err
			}

			ret[k] = v2
		}

		return ret, nil

	case []any:
		ret := []any{}

		for _, v := range objType {
			v2, err := normalize(v)
			if err != nil {
				return nil, err
			}

			ret = append(ret, v2)
		}

		return ret, nil

	default:
		return objType, nil
	}
}
