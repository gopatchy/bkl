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
	path, obj := popStringValue(obj, "$merge")
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

	path, obj = popStringValue(obj, "$replace")
	if path != "" {
		next := get(mergeFrom, path)
		if next == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrReplaceRefNotFound)
		}

		return Process(next, mergeFrom)
	}

	output, obj := popBoolValue(obj, "$output", false)
	if output {
		return nil, nil
	}

	encode, obj := popStringValue(obj, "$encode")

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
	path, obj := listPopStringValue(obj, "$merge")
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

	path, obj = listPopStringValue(obj, "$replace")
	if path != "" {
		next := get(mergeFrom, path)
		if next == nil {
			return nil, fmt.Errorf("%s: (%w)", path, ErrReplaceRefNotFound)
		}

		return Process(next, mergeFrom)
	}

	if listHasBoolValue(obj, "$output", false) {
		return nil, nil
	}

	encode, obj := listPopStringValue(obj, "$encode")

	obj, err := filterList(obj, func(v any) ([]any, error) {
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
