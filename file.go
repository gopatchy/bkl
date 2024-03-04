package bkl

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.jetpack.io/typeid"
)

type filePrefix struct{}

func (filePrefix) Prefix() string { return "file" }

type fileID struct {
	typeid.TypeID[filePrefix]
}

type file struct {
	id    fileID
	child *file
	path  string
	docs  []*Document
}

func (p *Parser) loadFile(path string, child *file) (*file, error) {
	f := &file{
		id:    typeid.Must(typeid.New[fileID]()),
		child: child,
		path:  path,
	}

	p.log("[%s] loading", f)

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
			if p.missingAsEmpty && errors.Is(err, os.ErrNotExist) {
				return f, nil
			}

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

		f.docs = append(f.docs, NewDocumentWithData(doc))
	}

	f.setParents()

	return f, nil
}

func (p *Parser) loadFileAndParents(path string, child *file) ([]*file, error) {
	f, err := p.loadFile(path, child)
	if err != nil {
		return nil, err
	}

	files := []*file{}

	parents, err := f.parents(p.missingAsEmpty)
	if err != nil {
		return nil, err
	}

	for _, parent := range parents {
		parentFiles, err := p.loadFileAndParents(parent, f)
		if err != nil {
			return nil, err
		}

		files = append(files, parentFiles...)
	}

	return append(files, f), nil
}

func (f *file) setParents() {
	if f.child == nil {
		return
	}

	for _, doc := range f.child.docs {
		doc.AddParents(f.docs...)
	}
}

func (f *file) parents(missingAsEmpty bool) ([]string, error) {
	parents, err := f.parentsFromDirective()
	if err != nil {
		return nil, err
	}

	if parents != nil {
		return parents, nil
	}

	parents, err = f.parentsFromSymlink(missingAsEmpty)
	if err != nil {
		return nil, err
	}

	if parents != nil {
		return parents, nil
	}

	return f.parentsFromFilename(missingAsEmpty)
}

func (f *file) parentsFromDirective() ([]string, error) {
	parents := []string{}
	noParent := false

	for _, doc := range f.docs {
		found, val := doc.PopMapValue("$parent")
		if !found {
			continue
		}

		switch val2 := val.(type) {
		case string:
			parents = append(parents, val2)

		case []any:
			val3, err := toStringList(val2)
			if err != nil {
				return nil, fmt.Errorf("$parent=%#v: %w", val2, ErrInvalidParent)
			}

			parents = append(parents, val3...)

		case bool:
			if val2 {
				return nil, fmt.Errorf("$parent=true: %w", ErrInvalidParent)
			}

			noParent = true

		case nil:
			noParent = true
		}
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

	return f.toAbsolutePaths(parents)
}

func (f *file) parentsFromSymlink(missingAsEmpty bool) ([]string, error) {
	if isStdin(f.path) {
		return nil, nil
	}

	dest, err := filepath.EvalSymlinks(f.path)
	if err != nil {
		if missingAsEmpty && errors.Is(err, os.ErrNotExist) {
			return f.parentsFromFilename(missingAsEmpty)
		}

		return nil, err
	}

	if dest == f.path {
		// Not a link
		return nil, nil
	}

	f.path = dest

	return f.parentsFromFilename(missingAsEmpty)
}

func (f *file) parentsFromFilename(missingAsEmpty bool) ([]string, error) {
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
			if missingAsEmpty {
				return []string{fmt.Sprintf("%s.%s", layerPath, parts[len(parts)-1])}, nil
			}

			return nil, fmt.Errorf("[%s]: %w", layerPath, ErrMissingFile)
		}

		return []string{extPath}, nil
	}
}

func (f *file) toAbsolutePaths(paths []string) ([]string, error) {
	ret := []string{}

	for _, path := range paths {
		path = filepath.Join(filepath.Dir(f.path), path)

		matches, err := globFiles(path)
		if err != nil {
			return nil, err
		}

		if len(matches) == 0 {
			return nil, fmt.Errorf("%s: %w", path, ErrMissingFile)
		}

		ret = append(ret, matches...)
	}

	return ret, nil
}

func (f *file) String() string {
	return f.path
}

func isStdin(path string) bool {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) == "-"
}
