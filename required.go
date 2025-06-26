package bkl

import (
	"fmt"
	"io/fs"
)

// RequiredFile loads a file and returns only the required fields and their ancestors.
// It expects the file to contain exactly one document.
// The file is loaded with MergeFileLayers but not processed, matching bklr behavior.
func (b *BKL) RequiredFile(fsys fs.FS, path string) (any, error) {
	// Create new parser for the file
	parser, err := New()
	if err != nil {
		return nil, err
	}

	realPath, _, err := parser.FileMatch(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("file %s: %w", path, err)
	}

	if err := parser.MergeFileLayers(fsys, realPath); err != nil {
		return nil, fmt.Errorf("merging %s: %w", path, err)
	}

	docs := parser.docs
	if len(docs) != 1 {
		return nil, fmt.Errorf("required operates on exactly 1 document, got %d in %s", len(docs), path)
	}

	return b.required(docs[0].Data)
}

// required returns only the required fields and their ancestors from the given value.
// It recursively traverses the value and keeps only paths that lead to "$required" markers.
func (b *BKL) required(obj any) (any, error) {
	return required(obj)
}

func required(obj any) (any, error) {
	switch obj2 := obj.(type) {
	case map[string]any:
		return requiredMap(obj2)

	case []any:
		return requiredList(obj2)

	case string:
		if obj2 == "$required" {
			return obj2, nil
		}

		return nil, nil

	default:
		return nil, nil
	}
}

func requiredMap(obj map[string]any) (any, error) {
	ret := map[string]any{}

	for k, v := range obj {
		v2, err := required(v)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			continue
		}

		ret[k] = v2
	}

	if len(ret) > 0 {
		return ret, nil
	}

	return nil, nil
}

func requiredList(obj []any) (any, error) {
	ret := []any{}

	for _, v := range obj {
		v2, err := required(v)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			continue
		}

		ret = append(ret, v2)
	}

	if len(ret) > 0 {
		return ret, nil
	}

	return nil, nil
}
