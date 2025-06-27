package bkl

import (
	"fmt"
	"io/fs"
)

// RequiredFile loads a file and returns only the required fields and their ancestors.
// It expects the file to contain exactly one document.
// The file is loaded directly without processing, matching bklr behavior.
func RequiredFile(fsys fs.FS, path string) (any, error) {
	// Create new parser for the file
	parser := &bkl{}

	realPath, _, err := fileMatch(fsys, path)
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
		return nil, fmt.Errorf("required operates on exactly 1 document, got %d in %s", len(docs), path)
	}

	return required(docs[0].Data)
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
