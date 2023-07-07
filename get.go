package bkl

import "strings"

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
