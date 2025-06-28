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

// Intersect loads multiple files and returns their intersection.
// It expects each file to contain exactly one document.
// The files are loaded directly without processing, matching bkli behavior.
// If format is nil, it infers the format from the formatPaths parameter.
func Intersect(fx fs.FS, paths []string, rootPath string, workingDir string, format *string, formatPaths ...*string) ([]byte, error) {
	preparedPaths, err := utils.PreparePathsForParser(paths, rootPath, workingDir)
	if err != nil {
		return nil, err
	}
	paths = preparedPaths
	if len(paths) < 2 {
		return nil, fmt.Errorf("intersect requires at least 2 files, got %d", len(paths))
	}

	var result any
	fx2 := fsys.New(fx)

	for i, path := range paths {

		var docs []*document.Document

		realPath, _, err := fileMatch(fx, path)
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

		if len(docs) != 1 {
			return nil, fmt.Errorf("intersect operates on exactly 1 document per file, got %d in %s", len(docs), path)
		}

		if i == 0 {
			result = docs[0].Data
			continue
		}

		result, err = intersect(result, docs[0].Data)
		if err != nil {
			return nil, err
		}
	}

	ft, err := determineFormat(format, formatPaths...)
	if err != nil {
		return nil, err
	}
	return ft.MarshalStream([]any{result})
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

		return "$required", nil
	}
}

func intersectMap(a map[string]any, b any) (any, error) {
	switch b2 := b.(type) {
	case map[string]any:
		return intersectMapMap(a, b2)

	default:
		// Different types but both defined
		return "$required", nil
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
		// Different types but both defined
		return "$required", nil
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

	if len(ret) == 0 {
		ret = append(ret, "$required")
	}

	return ret, nil
}
