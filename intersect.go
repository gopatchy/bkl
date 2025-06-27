package bkl

import (
	"fmt"
	"io/fs"
	"reflect"
)

// IntersectFiles loads multiple files and returns their intersection.
// It expects each file to contain exactly one document.
// The files are loaded directly without processing, matching bkli behavior.
func IntersectFiles(fsys fs.FS, paths []string) (any, error) {
	if len(paths) < 2 {
		return nil, fmt.Errorf("intersect requires at least 2 files, got %d", len(paths))
	}

	var result any

	for i, path := range paths {
		// Create new parser for each file
		parser, err := New()
		if err != nil {
			return nil, err
		}

		realPath, _, err := FileMatch(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("file %s: %w", path, err)
		}

		// Load file directly without processing
		fileSystem := newFS(fsys)
		fileObjs, err := loadFileAndParents(fileSystem, realPath, nil)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", path, err)
		}

		for _, f := range fileObjs {
			err := parser.mergeFileObj(f)
			if err != nil {
				return nil, fmt.Errorf("merging %s: %w", path, err)
			}
		}

		docs := parser.docs
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

	return result, nil
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
