package bkl

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func get(obj any, docs []any, m any) (any, error) {
	switch m2 := m.(type) {
	case string:
		return getPathFromString(obj, docs, m2)

	case []any:
		return getPathFromList(obj, docs, m2)

	case map[string]any:
		return getCross(docs, m2)

	default:
		return nil, fmt.Errorf("%T as reference: %w", m, ErrInvalidType)
	}
}

func getPathFromList(obj any, docs []any, path []any) (any, error) {
	if len(path) > 0 {
		pat, ok := path[0].(map[string]any)
		if ok {
			path = path[1:]

			var err error

			obj, err = getCrossDoc(docs, pat)
			if err != nil {
				return nil, err
			}
		}
	}

	path2, err := toStringList(path)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", path, err)
	}

	return getPath(obj, path2)
}

func getPathFromString(obj any, docs []any, path string) (any, error) {
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
		return getPathFromList(obj, docs, path3)

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
	pat, _ := popMapValue(conf, "$match")
	if pat == nil {
		return nil, fmt.Errorf("%#v: %w", conf, ErrMissingMatch)
	}

	doc, err := getCrossDoc(docs, pat)
	if err != nil {
		return nil, err
	}

	path, _ := popMapValue(conf, "$path")
	if path != nil {
		return get(doc, docs, path)
	}

	return doc, nil
}

func getCrossDoc(docs []any, pat any) (any, error) {
	var ret any

	for _, doc := range docs {
		if match(doc, pat) {
			if ret != nil {
				return nil, fmt.Errorf("%#v: %w", pat, ErrMultiMatch)
			}

			ret = doc
		}
	}

	if ret == nil {
		return nil, fmt.Errorf("%#v: %w", pat, ErrNoMatchFound)
	}

	return ret, nil
}
