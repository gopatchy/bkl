package bkl

import (
	"fmt"
	"strings"
)

func process1(obj any, mergeFrom *document, mergeFromDocs []*document, depth int) (any, error) {
	depth++

	if depth > 1000 {
		return nil, fmt.Errorf("%#v: %w", obj, ErrCircularRef)
	}

	switch obj2 := obj.(type) {
	case map[string]any:
		return process1Map(obj2, mergeFrom, mergeFromDocs, depth)

	case []any:
		return process1List(obj2, mergeFrom, mergeFromDocs, depth)

	case string:
		return process1String(obj2, mergeFrom, mergeFromDocs, depth)

	default:
		return obj, nil
	}
}

func process1Map(obj map[string]any, mergeFrom *document, mergeFromDocs []*document, depth int) (any, error) {
	// Not copying obj before merge preserves the layering behavior that
	// tests/merge-race relies upon.
	if v, found := obj["$merge"]; found {
		delete(obj, "$merge")
		return process1MapMerge(obj, mergeFrom, mergeFromDocs, v, depth)
	}

	if found, v, obj := popMapValue(obj, "$replace"); found {
		return process1MapReplace(obj, mergeFrom, mergeFromDocs, v, depth)
	}

	return filterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, err := process1(v, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		k2, err := process1(k, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		return map[string]any{k2.(string): v2}, nil
	})
}

func process1MapMerge(obj map[string]any, mergeFrom *document, mergeFromDocs []*document, v any, depth int) (any, error) {
	in, err := get(mergeFrom, mergeFromDocs, v)
	if err != nil {
		return nil, err
	}

	next, err := mergeMap(obj, in)
	if err != nil {
		return nil, err
	}

	return process1(next, mergeFrom, mergeFromDocs, depth)
}

func process1MapReplace(obj map[string]any, mergeFrom *document, mergeFromDocs []*document, v any, depth int) (any, error) {
	next, err := get(mergeFrom, mergeFromDocs, v)
	if err != nil {
		return nil, err
	}

	return process1(next, mergeFrom, mergeFromDocs, depth)
}

func process1List(obj []any, mergeFrom *document, mergeFromDocs []*document, depth int) (any, error) {
	merge := []any{}

	obj, err := filterList(obj, func(v any) ([]any, error) {
		v2, ok := v.(map[string]any)
		if !ok {
			return []any{v}, nil
		}

		if len(v2) != 1 {
			return []any{v}, nil
		}

		found, val, v2 := popMapValue(v2, "$merge")
		if found {
			merge = append(merge, val)
			return nil, nil
		}

		return []any{v2}, nil
	})

	for i, m := range merge {
		result, err := process1ListMerge(obj, mergeFrom, mergeFromDocs, m, depth)
		if err != nil {
			return nil, err
		}

		objList, ok := result.([]any)
		if !ok {
			if i < len(merge)-1 {
				return nil, fmt.Errorf("$merge replaced list with %T, but more $merge directives remain", result)
			}
			return result, nil
		}
		obj = objList
	}

	m, obj, err := popListMapValue(obj, "$replace")
	if err != nil {
		return nil, err
	}

	if m != nil {
		return process1ListReplace(obj, mergeFrom, mergeFromDocs, m, depth)
	}

	return filterList(obj, func(v any) ([]any, error) {
		v2, err := process1(v, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		return []any{v2}, nil
	})
}

func process1ListMerge(obj []any, mergeFrom *document, mergeFromDocs []*document, m any, depth int) (any, error) {
	in, err := get(mergeFrom, mergeFromDocs, m)
	if err != nil {
		return nil, err
	}

	return mergeList(obj, in)
}

func process1ListReplace(obj []any, mergeFrom *document, mergeFromDocs []*document, m any, depth int) (any, error) {
	next, err := get(mergeFrom, mergeFromDocs, m)
	if err != nil {
		return nil, err
	}

	return process1(next, mergeFrom, mergeFromDocs, depth)
}

func process1String(obj string, mergeFrom *document, mergeFromDocs []*document, depth int) (any, error) {
	if strings.HasPrefix(obj, "$merge:") {
		return process1StringMerge(obj, mergeFrom, mergeFromDocs, depth)
	}

	if strings.HasPrefix(obj, "$replace:") {
		return process1StringReplace(obj, mergeFrom, mergeFromDocs, depth)
	}

	return obj, nil
}

func process1StringMerge(obj string, mergeFrom *document, mergeFromDocs []*document, depth int) (any, error) {
	path := strings.TrimPrefix(obj, "$merge:")

	in, err := get(mergeFrom, mergeFromDocs, path)
	if err != nil {
		return nil, err
	}

	return process1(in, mergeFrom, mergeFromDocs, depth)
}

func process1StringReplace(obj string, mergeFrom *document, mergeFromDocs []*document, depth int) (any, error) {
	path := strings.TrimPrefix(obj, "$replace:")

	in, err := get(mergeFrom, mergeFromDocs, path)
	if err != nil {
		return nil, err
	}

	return process1(in, mergeFrom, mergeFromDocs, depth)
}
