package bkl

import (
	"errors"
	"fmt"
	"strings"
)

func process(root any) (any, bool, error) {
	return processRecursive(root, root)
}

func processRecursive(root any, obj any) (any, bool, error) {
	switch objType := obj.(type) {
	case map[string]any:
		if path, found := objType["$merge"]; found {
			delete(objType, "$merge")

			pathVal, ok := path.(string)
			if !ok {
				return nil, false, fmt.Errorf("%T: %w", path, ErrInvalidMergeType)
			}

			in := get(root, pathVal)
			if in == nil {
				return nil, false, fmt.Errorf("%s: (%w)", pathVal, ErrMergeRefNotFound)
			}

			next, err := merge(objType, in)
			if err != nil {
				return nil, false, err
			}

			return processRecursive(root, next)
		}

		if path, found := objType["$replace"]; found {
			delete(objType, "$replace")

			pathVal, ok := path.(string)
			if !ok {
				return nil, false, fmt.Errorf("%T: %w", path, ErrInvalidReplaceType)
			}

			next := get(root, pathVal)
			if next == nil {
				return nil, false, fmt.Errorf("%s: (%w)", pathVal, ErrReplaceRefNotFound)
			}

			return processRecursive(root, next)
		}

		if v, found := objType["$output"]; found {
			if v2, ok := v.(bool); ok && !v2 {
				return nil, false, nil
			}
		}

		encode := ""

		if v, found := objType["$encode"]; found {
			v2, ok := v.(string)
			if !ok {
				return nil, false, fmt.Errorf("%T: %w", v, ErrInvalidEncodeType)
			}

			encode = v2

			delete(objType, "$encode")
		}

		ret := map[string]any{}

		for k, v := range objType {
			v2, use, err := processRecursive(root, v)
			if err != nil {
				return nil, false, err
			}

			if use {
				ret[k] = v2
			}
		}

		if encode != "" {
			f, found := formatByExtension[encode]
			if !found {
				return nil, false, fmt.Errorf("%s: %w", encode, ErrUnknownFormat)
			}

			enc, err := f.encode(ret)
			if err != nil {
				return nil, false, errors.Join(ErrEncode, err)
			}

			return string(enc), true, nil
		}

		return ret, true, nil

	case []any:
		ret := []any{}

		for _, v := range objType {
			v2, use, err := processRecursive(root, v)
			if err != nil {
				return nil, false, err
			}

			if use {
				ret = append(ret, v2)
			}
		}

		return ret, true, nil

	case string:
		if strings.HasPrefix(objType, "$merge:") {
			path := strings.TrimPrefix(objType, "$merge:")

			in := get(root, path)
			if in == nil {
				return nil, false, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
			}

			return in, true, nil
		}

		if strings.HasPrefix(objType, "$replace:") {
			path := strings.TrimPrefix(objType, "$replace:")

			in := get(root, path)
			if in == nil {
				return nil, false, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
			}

			return in, true, nil
		}

		return obj, true, nil

	default:
		return obj, true, nil
	}
}
