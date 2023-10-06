package bkl

import (
	"fmt"

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

func popMapNilValue(m map[string]any, k string) (bool, map[string]any) {
	if hasMapNilValue(m, k) {
		m = polyfill.MapsClone(m)
		delete(m, k)

		return true, m
	}

	return false, m
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

func popListMapValue(l []any, k string) (any, []any, error) {
	var ret any

	l, err := filterList(l, func(x any) ([]any, error) {
		xMap, ok := x.(map[string]any)
		if !ok {
			return []any{x}, nil
		}

		val, xMap := popMapValue(xMap, k)
		if val != nil {
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

		if hasMapBoolValue(xMap, k, v) {
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

func popListMapStringValue(l []any, k string) (string, []any, error) {
	v2 := getListMapStringValue(l, k)

	if v2 == "" {
		return "", l, nil
	}

	l, err := filterList(l, func(x any) ([]any, error) {
		xMap, ok := x.(map[string]any)
		if !ok {
			return []any{x}, nil
		}

		v3, xMap := popMapStringValue(xMap, k)
		if v3 != "" {
			if len(xMap) > 0 {
				return nil, fmt.Errorf("%#v: %w", xMap, ErrExtraKeys)
			}

			return nil, nil
		}

		return []any{x}, nil
	})
	if err != nil {
		return "", nil, err
	}

	return v2, l, nil
}

func filterMap(m map[string]any, filter func(string, any) (map[string]any, error)) (map[string]any, error) {
	ret := map[string]any{}

	ks := polyfill.MapsKeys(m)
	polyfill.SlicesSort(ks)

	for _, k := range ks {
		v := m[k]

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
