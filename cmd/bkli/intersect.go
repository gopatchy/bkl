package main

import "reflect"

func intersect(a, b any) (any, error) {
	if b == nil {
		return nil, nil
	}

	switch a2 := a.(type) {
	case map[string]any:
		return intersectMap(a2, b)

	case []any:
		return intersectList(a2, b)

	case nil:
		return nil, nil

	default:
		if a == b {
			return a, nil
		}

		return "$required", nil
	}
}

func intersectMap(a map[string]any, b any) (any, error) {
	switch b2 := b.(type) {
	case map[string]any:
		return intersectMapMap(a, b2)

	default:
		// Different types but both defined
		return "$required", nil
	}
}

func intersectMapMap(a, b map[string]any) (map[string]any, error) {
	ret := map[string]any{}

	for k, v := range a {
		v2, found := b[k]

		if !found {
			continue
		}

		if v == nil && v2 == nil {
			ret[k] = nil
			continue
		}

		v2, err := intersect(v, v2)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			continue
		}

		ret[k] = v2
	}

	return ret, nil
}

func intersectList(a []any, b any) (any, error) {
	switch b2 := b.(type) {
	case []any:
		return intersectListList(a, b2)

	default:
		// Different types but both defined
		return "$required", nil
	}
}

func intersectListList(a, b []any) ([]any, error) { //nolint:unparam
	ret := []any{}

	for _, v1 := range a {
		for _, v2 := range b {
			if reflect.DeepEqual(v1, v2) {
				ret = append(ret, v1)
			}
		}
	}

	if len(ret) == 0 {
		ret = append(ret, "$required")
	}

	return ret, nil
}
