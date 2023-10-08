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

func loadFile(path string) (*file, error) {
	f := &file{
		path: path,
	}

	format, err := GetFormat(ext(path))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var fh io.ReadCloser

	if isStdin(path) {
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

		doc, err = env(doc)
		if err != nil {
			return nil, fmt.Errorf("[doc%d]: %w", i, err)
		}

		f.docs = append(f.docs, doc)
	}

	return f, nil
}

func (f *file) parents() ([]string, error) {
	parents, err := f.parentsFromDirective()
	if err != nil {
		return nil, err
	}

	if parents != nil {
		return parents, nil
	}

	parents, err = f.parentsFromSymlink()
	if err != nil {
		return nil, err
	}

	if parents != nil {
		return parents, nil
	}

	return f.parentsFromFilename()
}

func (f *file) parentsFromDirective() ([]string, error) {
	parents := []string{}
	noParent := false

	for _, doc := range f.docs {
		docMap, ok := doc.(map[string]any)
		if !ok {
			continue
		}

		if hasMapNilValue(docMap, "$parent") || hasMapBoolValue(docMap, "$parent", false) {
			delete(docMap, "$parent")
			noParent = true
		}

		parent := getMapStringValue(docMap, "$parent")
		if parent == "" {
			continue
		}

		delete(docMap, "$parent")

		parent = filepath.Join(filepath.Dir(f.path), parent)

		parentPath := findFile(parent)
		if parentPath == "" {
			return nil, fmt.Errorf("%s: %w", parent, ErrMissingFile)
		}

		parents = append(parents, parentPath)
	}

	if noParent {
		if len(parents) > 0 {
			return nil, fmt.Errorf("$parent=false and $parent=<string> in same file: %w", ErrConflictingParent)
		}

		return []string{}, nil
	}

	if len(parents) == 0 {
		return nil, nil
	}

	return parents, nil
}

func (f *file) parentsFromSymlink() ([]string, error) {
	if isStdin(f.path) {
		return nil, nil
	}

	dest, err := filepath.EvalSymlinks(f.path)
	if err != nil {
		return nil, err
	}

	if dest == f.path {
		// Not a link
		return nil, nil
	}

	f.path = dest

	return f.parentsFromFilename()
}

func (f *file) parentsFromFilename() ([]string, error) {
	if isStdin(f.path) {
		return []string{}, nil
	}

	dir := filepath.Dir(f.path)
	base := filepath.Base(f.path)

	parts := strings.Split(base, ".")
	// Last part is file extension

	switch {
	case len(parts) < 2:
		return nil, fmt.Errorf("[%s] %w", f.path, ErrInvalidFilename)

	case len(parts) == 2:
		return []string{}, nil

	default:
		layerPath := filepath.Join(dir, strings.Join(parts[:len(parts)-2], "."))

		extPath := findFile(layerPath)
		if extPath == "" {
			return nil, fmt.Errorf("[%s]: %w", layerPath, ErrMissingFile)
		}

		return []string{extPath}, nil
	}
}

func isStdin(path string) bool {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) == "-"
}
