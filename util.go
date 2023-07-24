package bkl

import (
	"github.com/gopatchy/bkl/polyfill"
)

func popMapValue(m map[string]any, k string) (any, map[string]any) {
	v, found := m[k]
	if !found {
		return nil, m
	}

	m = polyfill.MapsClone(m)
	delete(m, k)

	return v, m
}

func hasMapNilValue(m map[string]any, k string) bool {
	v, found := m[k]
	if !found {
		return false
	}

	return v == nil
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
		m = polyfill.MapsClone(m)
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

func getMapStringValue(m map[string]any, k string) string {
	v, found := m[k]
	if !found {
		return ""
	}

	return toString(v)
}

func popMapStringValue(m map[string]any, k string) (string, map[string]any) {
	v := getMapStringValue(m, k)

	if v != "" {
		m = polyfill.MapsClone(m)
		delete(m, k)
	}

	return v, m
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

func hasListMapBoolValue(l []any, k string, v bool) bool {
	for _, x := range l {
		xMap, ok := x.(map[string]any)
		if !ok {
			continue
		}

		if hasMapBoolValue(xMap, k, v) {
			return true
		}
	}

	return false
}

func popListMapBoolValue(l []any, k string, v bool) (bool, []any) {
	if !hasListMapBoolValue(l, k, v) {
		return false, l
	}

	l, _ = filterList(l, func(x any) ([]any, error) {
		xMap, ok := x.(map[string]any)
		if !ok {
			return []any{x}, nil
		}

		if hasMapBoolValue(xMap, k, v) {
			return nil, nil
		}

		return []any{x}, nil
	})

	return true, l
}

func getListMapStringValue(l []any, k string) string {
	for _, x := range l {
		xMap, ok := x.(map[string]any)
		if !ok {
			continue
		}

		v2 := getMapStringValue(xMap, k)
		if v2 != "" {
			return v2
		}
	}

	return ""
}

func popListMapStringValue(l []any, k string) (string, []any) {
	v2 := getListMapStringValue(l, k)

	if v2 == "" {
		return "", l
	}

	l, _ = filterList(l, func(x any) ([]any, error) {
		xMap, ok := x.(map[string]any)
		if !ok {
			return []any{x}, nil
		}

		if getMapStringValue(xMap, k) == "" {
			return []any{x}, nil
		}

		return nil, nil
	})

	return v2, l
}

func filterMap(m map[string]any, filter func(string, any) (map[string]any, error)) (map[string]any, error) {
	ret := map[string]any{}

	for k, v := range m {
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
