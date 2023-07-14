package bkl

import (
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

func findOutputs(obj any) []any {
	switch objType := obj.(type) {
	case map[string]any:
		ret := []any{}

		if v, found := objType["$output"]; found {
			if v2, ok := v.(bool); ok && v2 {
				delete(objType, "$output")

				ret = append(ret, obj)
			}
		}

		keys := maps.Keys(objType)
		slices.Sort(keys)

		for _, k := range keys {
			ret = append(ret, findOutputs(objType[k])...)
		}

		return ret

	case []any:
		ret := []any{}

		for _, v := range objType {
			ret = append(ret, findOutputs(v)...)
		}

		return ret

	default:
		return []any{}
	}
}
