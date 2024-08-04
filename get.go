package bkl

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func get(doc *Document, docs []*Document, m any) (any, error) {
	switch m2 := m.(type) {
	case string:
		ret, err := getPathFromString(doc.Data, docs, m2)
		if err != nil {
			return getVar(doc, m2)
		}

		return ret, nil

	case []any:
		return getPathFromList(doc.Data, docs, m2)

	case map[string]any:
		return getCross(docs, m2)

	default:
		return nil, fmt.Errorf("%T as reference: %w", m, ErrInvalidType)
	}
}

func getPathFromList(obj any, docs []*Document, path []any) (any, error) {
	if len(path) > 0 {
		var pat any

		pat, ok := path[0].(map[string]any)

		if !ok {
			pat, ok = path[0].([]any)
		}

		if ok {
			path = path[1:]

			doc, err := getCrossDoc(docs, pat)
			if err != nil {
				return nil, err
			}

			obj = doc.Data
		}
	}

	path2, err := toStringList(path)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", path, err)
	}

	return getPath(obj, path2)
}

func getPathFromString(obj any, docs []*Document, path string) (any, error) {
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

	switch obj2 := obj.(type) {
	case map[string]any:
		val, found := obj2[parts[0]]
		if !found {
			return nil, fmt.Errorf("%v: %w", parts, ErrRefNotFound)
		}

		return getPath(val, parts[1:])

	default:
		return nil, fmt.Errorf("%v: %w", parts, ErrRefNotFound)
	}
}

func getCross(docs []*Document, conf map[string]any) (any, error) {
	found, pat, _ := popMapValue(conf, "$match")
	if !found {
		return nil, fmt.Errorf("%#v: %w", conf, ErrMissingMatch)
	}

	doc, err := getCrossDoc(docs, pat)
	if err != nil {
		return nil, err
	}

	found, path, _ := popMapValue(conf, "$path")
	if found {
		return get(doc, docs, path)
	}

	return doc.Data, nil
}

func getCrossDoc(docs []*Document, pat any) (*Document, error) {
	var ret *Document

	for _, doc := range docs {
		if matchDoc(doc, pat) {
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

func getVar(doc *Document, name string) (any, error) {
	for k, v := range doc.Vars {
		if name == k {
			return v, nil
		}
	}

	return nil, fmt.Errorf("%s: %w", name, ErrVariableNotFound)
}
