package bkl

import (
	"errors"
	"fmt"
	"strings"
)

func process(root any) (any, error) {
	return processRecursive(root, root)
}

func processRecursive(root any, obj any) (any, error) {
	switch objType := obj.(type) {
	case map[string]any:
		return processMap(root, objType)

	case []any:
		return processList(root, objType)

	case string:
		return processString(root, objType)

	default:
		return obj, nil
	}
}

func processMap(root any, obj map[string]any) (any, error) {
	path := getStringValue(obj, "$merge")
	if path != "" {
		delete(obj, "$merge")

		in := get(root, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		next, err := merge(obj, in)
		if err != nil {
			return nil, err
		}

		return processRecursive(root, next)
	}

	path = getStringValue(obj, "$replace")
	if path != "" {
		delete(obj, "$replace")

		next := get(root, path)
		if next == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrReplaceRefNotFound)
		}

		return processRecursive(root, next)
	}

	if hasBoolValue(obj, "$output", false) {
		return nil, nil
	}

	encode := getStringValue(obj, "$encode")
	if encode != "" {
		delete(obj, "$encode")
	}

	ret := map[string]any{}

	for k, v := range obj {
		v2, err := processRecursive(root, v)
		if err != nil {
			return nil, err
		}

		if v2 != nil {
			ret[k] = v2
		}
	}

	if encode != "" {
		f, err := getFormat(encode)
		if err != nil {
			return nil, err
		}

		enc, err := f.encode(ret)
		if err != nil {
			return nil, errors.Join(ErrEncode, err)
		}

		return string(enc), nil
	}

	return ret, nil
}

func processList(root any, obj []any) (any, error) {
	ret := []any{}

	// TODO: Support $merge, $replace

	encode := ""

	for _, v := range obj {
		if vMap, ok := v.(map[string]any); ok {
			if encode2 := getStringValue(vMap, "$encode"); encode2 != "" {
				encode = encode2
				continue
			}

			if hasBoolValue(vMap, "$output", false) {
				return nil, nil
			}
		}

		v2, err := processRecursive(root, v)
		if err != nil {
			return nil, err
		}

		if v2 != nil {
			ret = append(ret, v2)
		}
	}

	if encode != "" {
		f, found := formatByExtension[encode]
		if !found {
			return nil, fmt.Errorf("%s: %w", encode, ErrUnknownFormat)
		}

		enc, err := f.encode(ret)
		if err != nil {
			return nil, errors.Join(ErrEncode, err)
		}

		return string(enc), nil
	}

	return ret, nil
}

func processString(root any, obj string) (any, error) {
	if strings.HasPrefix(obj, "$merge:") {
		path := strings.TrimPrefix(obj, "$merge:")

		in := get(root, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		return in, nil
	}

	if strings.HasPrefix(obj, "$replace:") {
		path := strings.TrimPrefix(obj, "$replace:")

		in := get(root, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		return in, nil
	}

	return obj, nil
}
