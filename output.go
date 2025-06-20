package bkl

func findOutputs(obj any) (any, []any, error) {
	switch obj2 := obj.(type) {
	case map[string]any:
		return findOutputsMap(obj2)

	case []any:
		return findOutputsList(obj2)

	default:
		return obj, []any{}, nil
	}
}

func findOutputsMap(obj map[string]any) (any, []any, error) {
	ret := map[string]any{}
	outs := []any{}

	output, obj := popMapBoolValue(obj, "$output", true)
	if output {
		outs = append(outs, ret)
	}

	for k, v := range sortedMap(obj) {
		vNew, subOuts, err := findOutputs(v)
		if err != nil {
			return nil, nil, err
		}

		outs = append(outs, subOuts...)
		ret[k] = vNew
	}

	return ret, outs, nil
}

func findOutputsList(obj []any) (any, []any, error) {
	ret := []any{}
	outs := []any{}

	output, obj, err := popListMapBoolValue(obj, "$output", true)
	if err != nil {
		return nil, nil, err
	}

	for _, v := range obj {
		vNew, subOuts, err := findOutputs(v)
		if err != nil {
			return nil, nil, err
		}

		outs = append(outs, subOuts...)
		ret = append(ret, vNew)
	}

	if output {
		outs = append(outs, any(ret))
	}

	return ret, outs, nil
}

func filterOutput(obj any) (any, error) {
	switch obj2 := obj.(type) {
	case map[string]any:
		return filterOutputMap(obj2)

	case []any:
		return filterOutputList(obj2)

	default:
		return obj, nil
	}
}

func filterOutputMap(obj map[string]any) (any, error) {
	output, obj := popMapBoolValue(obj, "$output", false)
	if output {
		return nil, nil
	}

	return filterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, err := filterOutput(v)
		if err != nil {
			return nil, err
		}

		return map[string]any{k: v2}, nil
	})
}

func filterOutputList(obj []any) (any, error) {
	output, obj, err := popListMapBoolValue(obj, "$output", false)
	if err != nil {
		return nil, err
	}

	if output {
		return nil, nil
	}

	return filterList(obj, func(v any) ([]any, error) {
		v2, err := filterOutput(v)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			return nil, nil
		}

		return []any{v2}, nil
	})
}
