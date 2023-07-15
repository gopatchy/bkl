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
