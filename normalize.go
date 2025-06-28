package bkl

import (
	"encoding/json"
	"fmt"

	"github.com/gopatchy/bkl/internal/utils"
	"github.com/gopatchy/bkl/pkg/errors"
)

func normalize(obj any) (any, error) {
	switch obj2 := obj.(type) {
	case map[any]any:
		return nil, fmt.Errorf("numeric keys not supported (%w)", errors.ErrInvalidType)

	case map[string]any:
		return normalizeMap(obj2)

	case []any:
		return normalizeList(obj2)

	case json.Number:
		return normalizeNumber(obj2)

	default:
		return obj2, nil
	}
}

func normalizeMap(obj map[string]any) (map[string]any, error) {
	return utils.FilterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, err := normalize(v)
		if err != nil {
			return nil, err
		}

		return map[string]any{k: v2}, nil
	})
}

func normalizeList(obj []any) ([]any, error) {
	return utils.FilterList(obj, func(v any) ([]any, error) {
		v2, err := normalize(v)
		if err != nil {
			return nil, err
		}

		return []any{v2}, nil
	})
}

func normalizeNumber(obj json.Number) (any, error) {
	if num, err := obj.Int64(); err == nil {
		if num == int64(int(num)) {
			return int(num), nil
		} else {
			return num, nil
		}
	}

	return obj.Float64()
}
