package bkl

func hasNilValue(m map[string]any, k string) bool {
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

func getBoolValue(m map[string]any, k string) (bool, bool) {
	v, found := m[k]
	if !found {
		return false, false
	}

	return toBool(v)
}

func hasBoolValue(m map[string]any, k string, v bool) bool {
	v2, ok := getBoolValue(m, k)
	if !ok {
		return false
	}

	return v2 == v
}

func popBoolValue(m map[string]any, k string, v bool) (bool, map[string]any) {
	found := hasBoolValue(m, k, v)

	if found {
		m = mapsClone(m)
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

func getStringValue(m map[string]any, k string) string {
	v, found := m[k]
	if !found {
		return ""
	}

	return toString(v)
}

func popStringValue(m map[string]any, k string) (string, map[string]any) {
	v := getStringValue(m, k)

	if v != "" {
		m = mapsClone(m)
		delete(m, k)
	}

	return v, m
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
