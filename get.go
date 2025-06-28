package bkl

import (
	"github.com/gopatchy/bkl/internal/document"
	"fmt"
	"strings"

	"github.com/gopatchy/bkl/internal/utils"
	"gopkg.in/yaml.v3"
)

func getWithVar(doc *document.Document, docs []*document.Document, ec *evalContext, m any) (any, error) {
	ret, err := get(doc, docs, m)
	if err != nil {
		switch m2 := m.(type) {
		case string:
			return ec.getVar(m2)

		default:
			return nil, err
		}
	}

	return ret, nil
}

func get(doc *document.Document, docs []*document.Document, m any) (any, error) {
	switch m2 := m.(type) {
	case string:
		return getPathFromString(doc.Data, docs, m2)

	case []any:
		return getPathFromList(doc.Data, docs, m2)

	case map[string]any:
		return getCross(docs, m2)

	default:
		return nil, fmt.Errorf("%T as reference: %w", m, ErrInvalidType)
	}
}

func getPathFromList(obj any, docs []*document.Document, path []any) (any, error) {
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

	path2, err := utils.ToStringList(path)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", path, err)
	}

	return getPath(obj, path2)
}

func getPathFromString(obj any, docs []*document.Document, path string) (any, error) {
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

func getCross(docs []*document.Document, conf map[string]any) (any, error) {
	found, pat, _ := utils.PopMapValue(conf, "$match")
	if !found {
		return nil, fmt.Errorf("%#v: %w", conf, ErrMissingMatch)
	}

	doc, err := getCrossDoc(docs, pat)
	if err != nil {
		return nil, err
	}

	found, path, _ := utils.PopMapValue(conf, "$path")
	if found {
		return get(doc, docs, path)
	}

	return doc.Data, nil
}

func getCrossDoc(docs []*document.Document, pat any) (*document.Document, error) {
	var ret *document.Document

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
