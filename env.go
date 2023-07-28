package bkl

import (
	"os"
	"strings"
)

func env(obj any) any {
	// TODO: Clean up
	// TODO: Missing env should be error
	switch objType := obj.(type) {
	case map[string]any:
		ret := map[string]any{}
		for k, v := range objType {
			ret[env(k).(string)] = env(v)
		}

		return ret

	case []any:
		ret := []any{}
		for _, v := range objType {
			ret = append(ret, env(v))
		}

		return ret

	case string:
		if !strings.HasPrefix(objType, "$env:") {
			return objType
		}

		return os.Getenv(strings.TrimPrefix(objType, "$env:"))

	default:
		return objType
	}
}
