package utils

import (
	"cmp"
	"iter"
	"maps"
	"slices"
)

func PopMapValue(m map[string]any, k string) (bool, any, map[string]any) {
	v, found := m[k]
	if !found {
		return false, nil, m
	}

	m = maps.Clone(m)
	delete(m, k)

	return true, v, m
}

func GetMapBoolValue(m map[string]any, k string) (bool, bool) {
	v, found := m[k]
	if !found {
		return false, false
	}

	return ToBool(v)
}

func HasMapBoolValue(m map[string]any, k string, v bool) bool {
	v2, ok := GetMapBoolValue(m, k)
	if !ok {
		return false
	}

	return v2 == v
}

func PopMapBoolValue(m map[string]any, k string, v bool) (bool, map[string]any) {
	found := HasMapBoolValue(m, k, v)

	if found {
		m = maps.Clone(m)
		delete(m, k)
	}

	return found, m
}

func GetMapIntValue(m map[string]any, k string) (int, bool) {
	v, found := m[k]
	if !found {
		return 0, false
	}

	return ToInt(v)
}

func SortedMap[Map ~map[K]V, K cmp.Ordered, V any](m Map) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, k := range slices.Sorted(maps.Keys(m)) {
			if !yield(k, m[k]) {
				return
			}
		}
	}
}

func FilterList(l []any, filter func(any) ([]any, error)) ([]any, error) {
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

func FilterMap(m map[string]any, filter func(string, any) (map[string]any, error)) (map[string]any, error) {
	ret := map[string]any{}

	for k, v := range SortedMap(m) {
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

func HasListMapBoolValue(l []any, k string, v bool) bool {
	for _, x := range l {
		xMap, ok := x.(map[string]any)
		if !ok {
			continue
		}

		if len(xMap) == 1 && HasMapBoolValue(xMap, k, v) {
			return true
		}
	}

	return false
}
