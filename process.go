package bkl

import (
	"fmt"
	"strings"
)

func Process(obj, mergeFrom any) (any, error) {
	switch objType := obj.(type) {
	case map[string]any:
		return processMap(objType, mergeFrom)

	case []any:
		return processList(objType, mergeFrom)

	case string:
		return processString(objType, mergeFrom)

	default:
		return obj, nil
	}
}

func processMap(obj map[string]any, mergeFrom any) (any, error) {
	path, obj := popMapStringValue(obj, "$merge")
	if path != "" {
		in := get(mergeFrom, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		next, err := mergeMap(obj, in)
		if err != nil {
			return nil, err
		}

		return Process(next, mergeFrom)
	}

	path, obj = popMapStringValue(obj, "$replace")
	if path != "" {
		next := get(mergeFrom, path)
		if next == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrReplaceRefNotFound)
		}

		return Process(next, mergeFrom)
	}

	output, obj := popMapBoolValue(obj, "$output", false)
	if output {
		return nil, nil
	}

	encode, obj := popMapStringValue(obj, "$encode")

	obj, err := filterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, err := Process(v, mergeFrom)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			return nil, nil
		}

		return map[string]any{k: v2}, nil
	})
	if err != nil {
		return nil, err
	}

	if encode != "" {
		f, err := GetFormat(encode)
		if err != nil {
			return nil, err
		}

		enc, err := f.Marshal(obj)
		if err != nil {
			return nil, err
		}

		return string(enc), nil
	}

	return obj, nil
}

func processList(obj []any, mergeFrom any) (any, error) {
	path, obj, err := popListMapStringValue(obj, "$merge")
	if err != nil {
		return nil, err
	}

	if path != "" {
		in := get(mergeFrom, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		next, err := mergeList(obj, in)
		if err != nil {
			return nil, err
		}

		return Process(next, mergeFrom)
	}

	path, obj, err = popListMapStringValue(obj, "$replace")
	if err != nil {
		return nil, err
	}

	if path != "" {
		next := get(mergeFrom, path)
		if next == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrReplaceRefNotFound)
		}

		return Process(next, mergeFrom)
	}

	if hasListMapBoolValue(obj, "$output", false) {
		return nil, nil
	}

	encode, obj, err := popListMapStringValue(obj, "$encode")
	if err != nil {
		return nil, err
	}

	obj, err = filterList(obj, func(v any) ([]any, error) {
		v2, err := Process(v, mergeFrom)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			return nil, nil
		}

		return []any{v2}, nil
	})
	if err != nil {
		return nil, err
	}

	if encode != "" {
		f, found := formatByExtension[encode]
		if !found {
			return nil, fmt.Errorf("%s: %w", encode, ErrUnknownFormat)
		}

		enc, err := f.Marshal(obj)
		if err != nil {
			return nil, err
		}

		return string(enc), nil
	}

	return obj, nil
}

func processString(obj string, mergeFrom any) (any, error) {
	if strings.HasPrefix(obj, "$merge:") {
		path := strings.TrimPrefix(obj, "$merge:")

		in := get(mergeFrom, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		return Process(in, mergeFrom)
	}

	if strings.HasPrefix(obj, "$replace:") {
		path := strings.TrimPrefix(obj, "$replace:")

		in := get(mergeFrom, path)
		if in == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrMergeRefNotFound)
		}

		return Process(in, mergeFrom)
	}

	return obj, nil
}
