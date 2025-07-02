package bkl

import (
	"fmt"
	"io/fs"
	"reflect"

	"github.com/gopatchy/bkl/internal/document"
	"github.com/gopatchy/bkl/internal/file"
	"github.com/gopatchy/bkl/internal/fsys"
	"github.com/gopatchy/bkl/internal/merge"
	"github.com/gopatchy/bkl/internal/utils"
)

func Intersect(fx fs.FS, paths []string, rootPath string, workingDir string, selector string, format *string, formatPaths ...*string) ([]byte, error) {
	preparedPaths, err := utils.PreparePathsForParser(paths, rootPath, workingDir)
	if err != nil {
		return nil, err
	}
	paths = preparedPaths
	if len(paths) < 2 {
		return nil, fmt.Errorf("intersect requires at least 2 files, got %d", len(paths))
	}

	fx2 := fsys.New(fx)

	tracking := map[string]any{}
	var keyOrder []string

	for i, path := range paths {
		var docs []*document.Document

		realPath, _, err := file.FileMatch(fx, path)
		if err != nil {
			return nil, fmt.Errorf("file %s: %w", path, err)
		}

		fileObjs, err := file.LoadAndParents(fx2, realPath, nil)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", path, err)
		}

		for _, f := range fileObjs {
			docs, err = merge.FileObj(docs, f)
			if err != nil {
				return nil, fmt.Errorf("merging %s: %w", path, err)
			}
		}

		if i == 0 {
			for _, doc := range docs {
				keyStr, err := evaluateSelector(doc, selector)
				if err != nil {
					return nil, fmt.Errorf("evaluating selector on document in %s: %w", path, err)
				}
				if _, exists := tracking[keyStr]; exists {
					return nil, fmt.Errorf("selector %q matches multiple documents in %s", keyStr, path)
				}
				tracking[keyStr] = doc.Data
				keyOrder = append(keyOrder, keyStr)
			}
		} else {
			seen := map[string]bool{}
			for _, doc := range docs {
				keyStr, err := evaluateSelector(doc, selector)
				if err != nil {
					return nil, fmt.Errorf("evaluating selector on document in %s: %w", path, err)
				}
				if seen[keyStr] {
					return nil, fmt.Errorf("selector %q matches multiple documents in %s", keyStr, path)
				}
				seen[keyStr] = true

				if existing, found := tracking[keyStr]; found {
					result, err := intersect(existing, doc.Data)
					if err != nil {
						return nil, err
					}
					tracking[keyStr] = result
				}
			}

			for key := range tracking {
				if !seen[key] {
					delete(tracking, key)
				}
			}
		}
	}

	results := []any{}
	for _, key := range keyOrder {
		if data, exists := tracking[key]; exists && data != nil {
			results = append(results, data)
		}
	}

	ft, err := determineFormat(format, formatPaths...)
	if err != nil {
		return nil, err
	}
	return ft.MarshalStream(results)
}

func intersect(a, b any) (any, error) {
	if b == nil {
		return nil, nil
	}

	switch a2 := a.(type) {
	case map[string]any:
		return intersectMap(a2, b)

	case []any:
		return intersectList(a2, b)

	case nil:
		return nil, nil

	default:
		if a == b {
			return a, nil
		}

		return nil, nil
	}
}

func intersectMap(a map[string]any, b any) (any, error) {
	switch b2 := b.(type) {
	case map[string]any:
		return intersectMapMap(a, b2)

	default:
		return nil, nil
	}
}

func intersectMapMap(a, b map[string]any) (map[string]any, error) {
	ret := map[string]any{}

	for k, v := range a {
		v2, found := b[k]

		if !found {
			continue
		}

		if v == nil && v2 == nil {
			ret[k] = nil
			continue
		}

		v2, err := intersect(v, v2)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			continue
		}

		ret[k] = v2
	}

	return ret, nil
}

func intersectList(a []any, b any) (any, error) {
	switch b2 := b.(type) {
	case []any:
		return intersectListList(a, b2)

	default:
		return nil, nil
	}
}

func intersectListList(a, b []any) ([]any, error) {
	ret := []any{}

	for _, v1 := range a {
		for _, v2 := range b {
			if reflect.DeepEqual(v1, v2) {
				ret = append(ret, v1)
			}
		}
	}

	// Empty list is valid, no $required marker

	return ret, nil
}
