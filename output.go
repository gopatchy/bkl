package bkl

import (
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func findOutputs(obj any) (any, []any) {
	switch objType := obj.(type) {
	case map[string]any:
		return findOutputsMap(objType)

	case []any:
		return findOutputsList(objType)

	default:
		return obj, []any{}
	}
}

func findOutputsMap(obj map[string]any) (any, []any) {
	ret := map[string]any{}
	outs := []any{}

	if hasBoolValue(obj, "$output", true) {
		outs = append(outs, ret)
	}

	keys := maps.Keys(obj)
	slices.Sort(keys)

	for _, k := range keys {
		if k == "$output" {
			continue
		}

		v := obj[k]

		vNew, subOuts := findOutputs(v)
		outs = append(outs, subOuts...)
		ret[k] = vNew
	}

	return ret, outs
}

func findOutputsList(obj []any) (any, []any) {
	ret := []any{}
	outs := []any{}
	output := false

	for _, v := range obj {
		if vMap, ok := v.(map[string]any); ok {
			if hasBoolValue(vMap, "$output", true) {
				output = true
				continue
			}
		}

		vNew, subOuts := findOutputs(v)
		outs = append(outs, subOuts...)
		ret = append(ret, vNew)
	}

	if output {
		outs = append(outs, any(ret))
	}

	return ret, outs
}
