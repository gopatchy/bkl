package output

import "github.com/gopatchy/bkl/internal/utils"

func FindOutputs(obj any) (any, []any, error) {
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

	output, obj := utils.PopMapBoolValue(obj, "$output", true)
	if output {
		outs = append(outs, ret)
	}

	for k, v := range utils.SortedMap(obj) {
		vNew, subOuts, err := FindOutputs(v)
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

	output, obj, err := utils.PopListMapBoolValue(obj, "$output", true)
	if err != nil {
		return nil, nil, err
	}

	for _, v := range obj {
		vNew, subOuts, err := FindOutputs(v)
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

func FilterOutput(obj any) (any, bool, error) {
	switch obj2 := obj.(type) {
	case map[string]any:
		return filterOutputMap(obj2)

	case []any:
		return filterOutputList(obj2)

	default:
		return obj, true, nil
	}
}

func filterOutputMap(obj map[string]any) (any, bool, error) {
	output, obj := utils.PopMapBoolValue(obj, "$output", false)
	if output {
		return nil, false, nil
	}

	filtered, err := utils.FilterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, include, err := FilterOutput(v)
		if err != nil {
			return nil, err
		}

		if !include {
			return map[string]any{}, nil
		}

		return map[string]any{k: v2}, nil
	})

	return filtered, true, err
}

func filterOutputList(obj []any) (any, bool, error) {
	output, obj, err := utils.PopListMapBoolValue(obj, "$output", false)
	if err != nil {
		return nil, false, err
	}

	if output {
		return nil, false, nil
	}

	filtered, err := utils.FilterList(obj, func(v any) ([]any, error) {
		v2, include, err := FilterOutput(v)
		if err != nil {
			return nil, err
		}

		if !include {
			return nil, nil
		}

		return []any{v2}, nil
	})

	return filtered, true, err
}
