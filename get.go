package bkl

import (
	"encoding/csv"
	"fmt"
	"strings"
)

func get(obj any, docs []any, m any) (any, error) {
	switch m2 := m.(type) {
	case string:
		return getPathFromString(obj, m2)

	case map[string]any:
		return getCross(docs, m2)

	default:
		return nil, fmt.Errorf("%T as reference: %w", m, ErrInvalidType)
	}
}

func getPathFromString(obj any, path string) (any, error) {
	r := csv.NewReader(strings.NewReader(path))
	r.Comma = '.'

	parts, err := r.Read()
	if err != nil {
		return nil, err
	}

	return getPath(obj, parts)
}

func getPath(obj any, parts []string) (any, error) {
	if len(parts) == 0 {
		return obj, nil
	}

	switch objType := obj.(type) {
	case map[string]any:
		return getPath(objType[parts[0]], parts[1:])

	default:
		return nil, fmt.Errorf("%s: %w", strings.Join(parts, "."), ErrRefNotFound)
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

	path, _ := popMapStringValue(conf, "$path")
	if path != "" {
		return getPathFromString(val, path)
	}

	return val, nil
}
