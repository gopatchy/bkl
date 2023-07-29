package bkl

import (
	"github.com/gopatchy/bkl/polyfill"
)

func findOutputs(obj any) (any, []any, error) {
	switch objType := obj.(type) {
	case map[string]any:
		return findOutputsMap(objType)

	case []any:
		return findOutputsList(objType)

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

	keys := polyfill.MapsKeys(obj)
	polyfill.SlicesSort(keys)

	for _, k := range keys {
		v := obj[k]

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
