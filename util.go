package bkl

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"

	"gopkg.in/yaml.v3"
)

func popMapValue(m map[string]any, k string) (bool, any, map[string]any) {
	v, found := m[k]
	if !found {
		return false, nil, m
	}

	m = maps.Clone(m)
	delete(m, k)

	return true, v, m
}

func toBool(a any) (bool, bool) {
	v, ok := a.(bool)
	return v, ok
}

func getMapBoolValue(m map[string]any, k string) (bool, bool) {
	v, found := m[k]
	if !found {
		return false, false
	}

	return toBool(v)
}

func hasMapBoolValue(m map[string]any, k string, v bool) bool {
	v2, ok := getMapBoolValue(m, k)
	if !ok {
		return false
	}

	return v2 == v
}

func popMapBoolValue(m map[string]any, k string, v bool) (bool, map[string]any) {
	found := hasMapBoolValue(m, k, v)

	if found {
		m = maps.Clone(m)
		delete(m, k)
	}

	return found, m
}

func toString(a any) string {
	v, ok := a.(string)
	if !ok {
		return ""
	}

	return v
}

func toInt(a any) (int, bool) {
	v, ok := a.(int)
	return v, ok
}

func getMapIntValue(m map[string]any, k string) (int, bool) {
	v, found := m[k]
	if !found {
		return 0, false
	}

	return toInt(v)
}

func popListString(l []any, v string) (bool, []any) {
	found := false

	l, _ = filterList(l, func(x any) ([]any, error) {
		s, ok := x.(string)
		if !ok {
			return []any{x}, nil
		}

		if s == v {
			found = true
			return nil, nil
		}

		return []any{x}, nil
	})

	return found, l
}

func popListMapValue(l []any, k string) (any, []any, error) {
	var ret any

	l, err := filterList(l, func(x any) ([]any, error) {
		xMap, ok := x.(map[string]any)
		if !ok {
			return []any{x}, nil
		}

		if len(xMap) != 1 {
			return []any{x}, nil
		}

		found, val, xMap := popMapValue(xMap, k)
		if found {
			if ret != nil {
				return nil, fmt.Errorf("%#v: %w", l, ErrExtraKeys)
			}

			ret = val

			return nil, nil
		}

		return []any{xMap}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return ret, l, nil
}

func hasListMapBoolValue(l []any, k string, v bool) bool {
	for _, x := range l {
		xMap, ok := x.(map[string]any)
		if !ok {
			continue
		}

		// Only match if map has exactly 1 key (the directive we're looking for)
		if len(xMap) == 1 && hasMapBoolValue(xMap, k, v) {
			return true
		}
	}

	return false
}

func popListMapBoolValue(l []any, k string, v bool) (bool, []any, error) {
	if !hasListMapBoolValue(l, k, v) {
		return false, l, nil
	}

	l, err := filterList(l, func(x any) ([]any, error) {
		xMap, ok := x.(map[string]any)
		if !ok {
			return []any{x}, nil
		}

		// Only match if map has exactly 1 key (the directive we're looking for)
		if len(xMap) != 1 {
			return []any{x}, nil
		}

		found, xMap := popMapBoolValue(xMap, k, v)
		if found {
			if len(xMap) > 0 {
				return nil, fmt.Errorf("%#v: %w", xMap, ErrExtraKeys)
			}

			return nil, nil
		}

		return []any{x}, nil
	})
	if err != nil {
		return false, nil, err
	}

	return true, l, nil
}

func sortedMap[Map ~map[K]V, K cmp.Ordered, V any](m Map) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, k := range slices.Sorted(maps.Keys(m)) {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}

func filterMap(m map[string]any, filter func(string, any) (map[string]any, error)) (map[string]any, error) {
	ret := map[string]any{}

	for k, v := range sortedMap(m) {
		m2, err := filter(k, v)
		if err != nil {
			return nil, err
		}

		for k2, v2 := range m2 {
			ret[k2] = v2
		}
	}

	return ret, nil
}

func filterList(l []any, filter func(any) ([]any, error)) ([]any, error) {
	ret := []any{}

	for _, v := range l {
		l2, err := filter(v)
		if err != nil {
			return nil, err
		}

		ret = append(ret, l2...)
	}

	return ret, nil
}

func toStringList(l []any) ([]string, error) {
	ret := []string{}

	for _, v := range l {
		switch v2 := v.(type) {
		case string:
			ret = append(ret, v2)

		default:
			return nil, fmt.Errorf("%T: %w", v, ErrInvalidType)
		}
	}

	return ret, nil
}

func toStringListPermissive(v any) ([]string, error) {
	v2, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%T: %w", v, ErrInvalidType)
	}

	ret := []string{}
	for _, v3 := range v2 {
		ret = append(ret, fmt.Sprintf("%v", v3))
	}

	return ret, nil
}

func deepClone(v any) (any, error) {
	yml, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}

	var ret any

	err = yaml.Unmarshal(yml, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
