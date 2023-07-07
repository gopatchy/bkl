package bkl

func match(obj any, pat any) bool {
	switch patType := pat.(type) {
	case map[string]any:
		objMap, ok := obj.(map[string]any)
		if !ok {
			return false
		}

		result := true

		for patKey, patVal := range patType {
			result = result && match(objMap[patKey], patVal)
		}

		return result

	case []any:
		objList, ok := obj.([]any)
		if !ok {
			return false
		}

		for _, patVal := range patType {
			found := false

			for _, objVal := range objList {
				if match(objVal, patVal) {
					found = true
					break
				}
			}

			if !found {
				return false
			}
		}

		return true

	default:
		return obj == pat
	}
}
