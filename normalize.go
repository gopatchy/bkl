package bkl

import (
	"fmt"
	"path/filepath"
	"strings"
)

func normalize(obj any, dir string) (any, error) {
	switch objType := obj.(type) {
	case map[any]any:
		return nil, fmt.Errorf("numeric keys not supported (%w)", ErrInvalidType)

	case []map[string]any:
		ret := []any{}

		for _, v := range objType {
			v2, err := normalize(v, dir)
			if err != nil {
				return nil, err
			}

			ret = append(ret, v2)
		}

		return ret, nil

	case map[string]any:
		ret := map[string]any{}

		for k, v := range objType {
			v2, err := normalize(v, dir)
			if err != nil {
				return nil, err
			}

			ret[k] = v2
		}

		return ret, nil

	case []any:
		ret := []any{}

		for _, v := range objType {
			v2, err := normalize(v, dir)
			if err != nil {
				return nil, err
			}

			ret = append(ret, v2)
		}

		return ret, nil

	case string:
		if strings.HasPrefix(objType, "$encode:") {
			path := strings.TrimPrefix(objType, "$encode:")
			return "$encode:" + filepath.Join(dir, path), nil
		}

		return objType, nil

	default:
		return objType, nil
	}
}
