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
		return processMap(root, objType)

	case []any:
		return processList(root, objType)

	case string:
		return processString(root, objType)

	default:
		return obj, true, nil
	}
}

func processMap(root any, obj map[string]any) (any, bool, error) {
	if path, found := obj["$merge"]; found {
		delete(obj, "$merge")

		pathVal, ok := path.(string)
		if !ok {
			return nil, false, fmt.Errorf("%T: %w", path, ErrInvalidMergeType)
		}

		in := get(root, pathVal)
		if in == nil {
			return nil, false, fmt.Errorf("%s: (%w)", pathVal, ErrMergeRefNotFound)
		}

		next, err := merge(obj, in)
		if err != nil {
			return nil, false, err
		}

		return processRecursive(root, next)
	}

	if path, found := obj["$replace"]; found {
		delete(obj, "$replace")

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

	if v, found := obj["$output"]; found {
		if v2, ok := v.(bool); ok && !v2 {
			return nil, false, nil
		}
	}

	encode := ""

	if v, found := obj["$encode"]; found {
		v2, ok := v.(string)
		if !ok {
			return nil, false, fmt.Errorf("%T: %w", v, ErrInvalidEncodeType)
		}

		encode = v2

		delete(obj, "$encode")
	}

	ret := map[string]any{}

	for k, v := range obj {
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
}

func processList(root any, obj []any) (any, bool, error) {
	ret := []any{}

	// TODO: Support $merge, $replace, $encode, $output

	for _, v := range obj {
		v2, use, err := processRecursive(root, v)
		if err != nil {
			return nil, false, err
		}

		if use {
			ret = append(ret, v2)
		}
	}

	return ret, true, nil
}

func processString(root any, obj string) (any, bool, error) {
	if strings.HasPrefix(obj, "$merge:") {
		path := strings.TrimPrefix(obj, "$merge:")

		in := get(root, path)
		if in == nil {
			return nil, false, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		return in, true, nil
	}

	if strings.HasPrefix(obj, "$replace:") {
		path := strings.TrimPrefix(obj, "$replace:")

		in := get(root, path)
		if in == nil {
			return nil, false, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		return in, true, nil
	}

	return obj, true, nil
}
