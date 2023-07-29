package bkl

import (
	"fmt"
	"os"
	"strings"
)

func env(obj any) (any, error) {
	// TODO: Missing env should be error
	switch objType := obj.(type) {
	case map[string]any:
		return envMap(objType)

	case []any:
		return envList(objType)

	case string:
		return envString(objType)

	default:
		return objType, nil
	}
}

func envMap(obj map[string]any) (map[string]any, error) {
	return filterMap(obj, func(k string, v any) (map[string]any, error) {
		k, err := envString(k)
		if err != nil {
			return nil, err
		}

		v, err = env(v)
		if err != nil {
			return nil, err
		}

		return map[string]any{k: v}, nil
	})
}

func envList(obj []any) ([]any, error) {
	return filterList(obj, func(v any) ([]any, error) {
		v, err := env(v)
		if err != nil {
			return nil, err
		}

		return []any{v}, nil
	})
}

func envString(obj string) (string, error) {
	if !strings.HasPrefix(obj, "$env:") {
		return obj, nil
	}

	v, found := os.LookupEnv(strings.TrimPrefix(obj, "$env:"))
	if !found {
		return "", fmt.Errorf("%s: %w", obj, ErrMissingEnv)
	}

	return v, nil
}
