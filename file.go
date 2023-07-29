package bkl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type file struct {
	path string
	docs []any
}

var baseTemplate = ""

func loadFile(path string) (*file, error) {
	f := &file{
		path: path,
	}

	format, err := GetFormat(ext(path))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var fh io.ReadCloser

	if strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) == "-" {
		fh = os.Stdin
	}

	if fh == nil {
		fh, err = os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		defer fh.Close()
	}

	raw, err := io.ReadAll(fh)
	if err != nil {
		return nil, err
	}

	docs, err := format.UnmarshalStream(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	for i, doc := range docs {
		doc, err = normalize(doc)
		if err != nil {
			return nil, fmt.Errorf("[doc%d]: %w", i, err)
		}

		doc = env(doc)

		f.docs = append(f.docs, doc)
	}

	return f, nil
}

func (f *file) parent() (*string, error) {
	parent, err := f.parentFromDirective()
	if err != nil {
		return nil, err
	}

	if parent != nil {
		return parent, nil
	}

	parent, err = f.parentFromSymlink()
	if err != nil {
		return nil, err
	}

	if parent != nil {
		return parent, nil
	}

	return f.parentFromFilename()
}

func (f *file) parentFromDirective() (*string, error) {
	docMap, ok := f.docs[0].(map[string]any)
	if !ok {
		return nil, nil
	}

	if hasMapNilValue(docMap, "$parent") || hasMapBoolValue(docMap, "$parent", false) {
		delete(docMap, "$parent")
		return &baseTemplate, nil
	}

	parent := getMapStringValue(docMap, "$parent")
	if parent == "" {
		return nil, nil
	}

	delete(docMap, "$parent")

	parent = filepath.Join(filepath.Dir(f.path), parent)

	parentPath := findFile(parent)
	if parentPath == "" {
		return nil, fmt.Errorf("%s: %w", parent, ErrMissingFile)
	}

	return &parentPath, nil
}

func (f *file) parentFromSymlink() (*string, error) {
	dest, err := filepath.EvalSymlinks(f.path)
	if err != nil {
		return nil, err
	}

	if dest == f.path {
		// Not a link
		return nil, nil
	}

	f.path = dest

	return f.parentFromFilename()
}

func (f *file) parentFromFilename() (*string, error) {
	dir := filepath.Dir(f.path)
	base := filepath.Base(f.path)

	parts := strings.Split(base, ".")
	// Last part is file extension

	switch {
	case len(parts) < 2:
		return nil, fmt.Errorf("[%s] %w", f.path, ErrInvalidFilename)

	case len(parts) == 2:
		return &baseTemplate, nil

	default:
		layerPath := filepath.Join(dir, strings.Join(parts[:len(parts)-2], "."))

		extPath := findFile(layerPath)
		if extPath == "" {
			return nil, fmt.Errorf("[%s]: %w", layerPath, ErrMissingFile)
		}

		return &extPath, nil
	}
}
