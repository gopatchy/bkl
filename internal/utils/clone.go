package utils

import (
	"sync"
)

// mapPool reuses maps to reduce allocations
var mapPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]any, 16)
	},
}

// DeepClone performs deep cloning without YAML marshaling
func DeepClone(v any) (any, error) {
	return deepCloneValue(v), nil
}

func deepCloneValue(v any) any {
	if v == nil {
		return nil
	}

	switch val := v.(type) {
	case map[string]any:
		return deepCloneMap(val)
	case []any:
		return deepCloneSlice(val)
	case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		// Primitive types are copied by value
		return val
	default:
		// For unknown types, return as-is (shallow copy)
		return val
	}
}

func deepCloneMap(m map[string]any) map[string]any {
	result := mapPool.Get().(map[string]any)
	// Clear the map
	for k := range result {
		delete(result, k)
	}

	for k, v := range m {
		result[k] = deepCloneValue(v)
	}

	// Create a new map to return (so we can reuse the pooled one)
	finalResult := make(map[string]any, len(result))
	for k, v := range result {
		finalResult[k] = v
	}

	mapPool.Put(result)
	return finalResult
}

func deepCloneSlice(s []any) []any {
	if len(s) == 0 {
		return nil
	}

	result := make([]any, len(s))
	for i, v := range s {
		result[i] = deepCloneValue(v)
	}
	return result
}
