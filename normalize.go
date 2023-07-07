package bkl

func normalize(obj any) any {
	switch objType := obj.(type) {
	case []map[string]any:
		ret := []any{}
		for _, v := range objType {
			ret = append(ret, normalize(v))
		}

		return ret

	case map[string]any:
		ret := map[string]any{}
		for k, v := range objType {
			ret[k] = normalize(v)
		}

		return ret

	case []any:
		ret := []any{}
		for _, v := range objType {
			ret = append(ret, normalize(v))
		}

		return ret

	default:
		return objType
	}
}
