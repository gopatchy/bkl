package bkl

import (
	"fmt"
)

func normalize(obj any) (any, error) {
	switch objType := obj.(type) {
	case map[any]any:
		return nil, fmt.Errorf("numeric keys not supported (%w)", ErrInvalidType)

	case []map[string]any:
		obj2 := []any{}

		for _, v := range objType {
			obj2 = append(obj2, v)
		}

		return normalize(obj2)

	case map[string]any:
		return filterMap(objType, func(k string, v any) (map[string]any, error) {
			v2, err := normalize(v)
			if err != nil {
				return nil, err
			}

			return map[string]any{k: v2}, nil
		})

	case []any:
		return filterList(objType, func(v any) ([]any, error) {
			v2, err := normalize(v)
			if err != nil {
				return nil, err
			}

			return []any{v2}, nil
		})

	default:
		return objType, nil
	}
}
