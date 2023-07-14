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

	keys := maps.Keys(obj)
	slices.Sort(keys)

	for _, k := range keys {
		v := obj[k]

		if k == "$output" {
			if v2, ok := v.(bool); ok && v2 {
				outs = append(outs, ret)
				continue
			}
		}

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
			if v2, found := vMap["$output"]; found {
				if v3, ok := v2.(bool); ok && v3 {
					output = true
					continue
				}
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
