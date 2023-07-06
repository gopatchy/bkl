package bkl

import "strings"

func canonicalizeType(in any) any {
	switch inType := in.(type) {
	case []map[string]any:
		ret := []any{}
		for _, val := range inType {
			ret = append(ret, val)
		}

		return ret

	default:
		return inType
	}
}

func get(obj any, path string) any {
	parts := strings.Split(path, ".")
	return getRecursive(obj, parts)
}

func getRecursive(obj any, parts []string) any {
	if len(parts) == 0 {
		return obj
	}

	switch objType := obj.(type) {
	case map[string]any:
		return getRecursive(objType[parts[0]], parts[1:])

	default:
		return nil
	}
}
