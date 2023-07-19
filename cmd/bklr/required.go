package main

func required(obj any) (any, error) {
	switch obj2 := obj.(type) {
	case map[string]any:
		return requiredMap(obj2)

	case []any:
		return requiredList(obj2)

	case string:
		if obj2 == "$required" {
			return obj2, nil
		}

		return nil, nil

	default:
		return nil, nil
	}
}

func requiredMap(obj map[string]any) (any, error) {
	ret := map[string]any{}

	for k, v := range obj {
		v2, err := required(v)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			continue
		}

		ret[k] = v2
	}

	if len(ret) > 0 {
		return ret, nil
	}

	return nil, nil
}

func requiredList(obj []any) (any, error) {
	ret := []any{}

	for _, v := range obj {
		v2, err := required(v)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			continue
		}

		ret = append(ret, v2)
	}

	if len(ret) > 0 {
		return ret, nil
	}

	return nil, nil
}
