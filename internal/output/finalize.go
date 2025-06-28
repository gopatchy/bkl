package output

import (
	"strings"
)

func FinalizeOutput(obj any) any {
	switch obj2 := obj.(type) {
	case map[string]any:
		return finalizeMap(obj2)

	case []any:
		return finalizeList(obj2)

	case string:
		return finalizeString(obj2)

	default:
		return obj
	}
}

func finalizeMap(obj map[string]any) map[string]any {
	newObj := make(map[string]any, len(obj))
	for k, v := range obj {
		newObj[finalizeString(k)] = FinalizeOutput(v)
	}

	return newObj
}

func finalizeList(obj []any) []any {
	newList := make([]any, len(obj))
	for idx, v := range obj {
		newList[idx] = FinalizeOutput(v)
	}

	return newList
}

func finalizeString(obj string) string {
	return strings.ReplaceAll(obj, "$$", "$")
}
