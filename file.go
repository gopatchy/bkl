package bkl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type file struct {
	id    string
	child *file
	path  string
	docs  []*Document
}

func (p *Parser) loadFile(path string, child *file) (*file, error) {
	f := &file{
		id:    path,
		child: child,
		path:  path,
	}

	if child != nil {
		f.id = fmt.Sprintf("%s|%s", child.id, f.id)
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
		fh, err = p.fsys.Open(path)
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
		id := fmt.Sprintf("%s|doc%d", f, i)

		doc, err = normalize(doc)
		if err != nil {
			return nil, fmt.Errorf("[%s]: %w", id, err)
		}

		docObj := NewDocumentWithData(id, doc)
		f.docs = append(f.docs, docObj)
	}

	f.setParents()

	return f, nil
}

func (p *Parser) loadFileAndParents(path string, child *file) ([]*file, error) {
	return p.loadFileAndParentsInt(path, child, []string{})
}

func (p *Parser) loadFileAndParentsInt(path string, child *file, stack []string) ([]*file, error) {
	if slices.Contains(stack, path) {
		return nil, fmt.Errorf("%s: %w", strings.Join(append(stack, path), " -> "), ErrCircularRef)
	}

	f, err := p.loadFile(path, child)
	if err != nil {
		return nil, err
	}

	stack = append(stack, path)

	parents, err := f.parents(p.fsys)
	if err != nil {
		return nil, err
	}

	files := []*file{}

	for _, parent := range parents {
		parentFiles, err := p.loadFileAndParentsInt(parent, f, stack)
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

func (f *file) parents(fsys *FS) ([]string, error) {
	parents, err := f.parentsFromDirective(fsys)
	if err != nil {
		return nil, err
	}

	if parents != nil {
		return parents, nil
	}

	return f.parentsFromFilename(fsys)
}

func (f *file) parentsFromDirective(fsys *FS) ([]string, error) {
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

	return f.toAbsolutePaths(fsys, parents)
}

func (f *file) parentsFromFilename(fsys *FS) ([]string, error) {
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

		extPath := fsys.findFile(layerPath)
		if extPath == "" {
			return nil, fmt.Errorf("[%s]: %w", layerPath, ErrMissingFile)
		}

		return []string{extPath}, nil
	}
}

func (f *file) toAbsolutePaths(fsys *FS, paths []string) ([]string, error) {
	ret := []string{}

	for _, path := range paths {
		path = filepath.Join(filepath.Dir(f.path), path)

		matches, err := fsys.globFiles(path)
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
	return f.id
}

func isStdin(path string) bool {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) == "-"
}
