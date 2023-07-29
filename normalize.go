package bkl

import (
	"fmt"
)

func normalize(obj any) (any, error) {
	switch obj2 := obj.(type) {
	case map[any]any:
		return nil, fmt.Errorf("numeric keys not supported (%w)", ErrInvalidType)

	case []map[string]any:
		return normalizeListMap(obj2)

	case map[string]any:
		return normalizeMap(obj2)

	case []any:
		return normalizeList(obj2)

	default:
		return obj2, nil
	}
}

func normalizeListMap(obj []map[string]any) ([]any, error) {
	obj2 := []any{}

	for _, v := range obj {
		obj2 = append(obj2, v)
	}

	return normalizeList(obj2)
}

func normalizeMap(obj map[string]any) (map[string]any, error) {
	return filterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, err := normalize(v)
		if err != nil {
			return nil, err
		}

		return map[string]any{k: v2}, nil
	})
}

func normalizeList(obj []any) ([]any, error) {
	return filterList(obj, func(v any) ([]any, error) {
		v2, err := normalize(v)
		if err != nil {
			return nil, err
		}

		return []any{v2}, nil
	})
}
