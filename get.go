package bkl

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func get(obj any, docs []any, m any) (any, error) {
	switch m2 := m.(type) {
	case string:
		return getPathFromString(obj, m2)

	case []any:
		return getPathFromList(obj, m2)

	case map[string]any:
		return getCross(docs, m2)

	default:
		return nil, fmt.Errorf("%T as reference: %w", m, ErrInvalidType)
	}
}

func getPathFromList(obj any, path []any) (any, error) {
	path2, err := toStringList(path)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", path, err)
	}

	return getPath(obj, path2)
}

func getPathFromString(obj any, path string) (any, error) {
	var path2 any
	err := yaml.Unmarshal([]byte(path), &path2)
	if err != nil {
		return nil, err
	}

	switch path3 := path2.(type) {
	case string:
		parts := strings.Split(path3, ".")
		return getPath(obj, parts)

	case []any:
		return getPathFromList(obj, path3)

	default:
		return nil, fmt.Errorf("%T as reference: %w", path2, ErrInvalidType)
	}
}

func getPath(obj any, parts []string) (any, error) {
	if len(parts) == 0 {
		return obj, nil
	}

	switch objType := obj.(type) {
	case map[string]any:
		return getPath(objType[parts[0]], parts[1:])

	default:
		return nil, fmt.Errorf("%v: %w", parts, ErrRefNotFound)
	}
}

func getCross(docs []any, conf map[string]any) (any, error) {
	m, _ := popMapValue(conf, "$match")
	if m == nil {
		return nil, fmt.Errorf("%#v: %w", conf, ErrMissingMatch)
	}

	var val any

	for _, doc := range docs {
		if match(doc, m) {
			if val != nil {
				return nil, fmt.Errorf("%#v: %w", m, ErrMultiMatch)
			}

			val = doc
		}
	}

	if val == nil {
		return nil, fmt.Errorf("%#v: %w", m, ErrNoMatchFound)
	}

	path, _ := popMapValue(conf, "$path")
	if path != nil {
		return get(val, docs, path)
	}

	return val, nil
}
