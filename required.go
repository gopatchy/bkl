package bkl

import (
	"fmt"
	"io/fs"

	"github.com/gopatchy/bkl/internal/fsys"
)

// Required loads a file and returns only the required fields and their ancestors.
// It expects the file to contain exactly one document.
// The file is loaded directly without processing, matching bklr behavior.
// If format is nil, it infers the format from the paths parameter.
func Required(fx fs.FS, path string, rootPath string, workingDir string, format *string, paths ...*string) ([]byte, error) {
	preparedPaths, err := preparePathsForParser([]string{path}, rootPath, workingDir)
	if err != nil {
		return nil, err
	}
	path = preparedPaths[0]
	parser := &bkl{}

	realPath, _, err := fileMatch(fx, path)
	if err != nil {
		return nil, fmt.Errorf("file %s: %w", path, err)
	}

	// Load file directly without processing
	fileObjs, err := loadFileAndParents(fsys.New(fx), realPath, nil)
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

	result, err := required(docs[0].Data)
	if err != nil {
		return nil, err
	}

	// Determine format and return formatted output
	ft, err := determineFormat(format, paths...)
	if err != nil {
		return nil, err
	}
	return ft.MarshalStream([]any{result})
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
