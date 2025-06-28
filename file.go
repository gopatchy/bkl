package bkl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gopatchy/bkl/internal/document"
	"github.com/gopatchy/bkl/internal/format"
	"github.com/gopatchy/bkl/internal/fsys"
	"github.com/gopatchy/bkl/internal/utils"
)

type file struct {
	id    string
	child *file
	path  string
	docs  []*document.Document
}

func loadFile(fsys *fsys.FS, path string, child *file) (*file, error) {
	f := &file{
		id:    path,
		child: child,
		path:  path,
	}

	if child != nil {
		f.id = fmt.Sprintf("%s|%s", child.id, f.id)
	}

	debugLog("[%s] loading", f)

	ft, err := format.Get(utils.Ext(path))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var fh io.ReadCloser

	if utils.IsStdin(path) {
		fh = os.Stdin
	}
	if fh == nil {
		fh, err = fsys.Open(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		defer fh.Close()
	}

	raw, err := io.ReadAll(fh)
	if err != nil {
		return nil, err
	}

	docs, err := ft.UnmarshalStream(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	for i, doc := range docs {
		id := fmt.Sprintf("%s|doc%d", f, i)

		doc, err = normalize(doc)
		if err != nil {
			return nil, fmt.Errorf("[%s]: %w", id, err)
		}

		docObj := document.NewWithData(id, doc)
		f.docs = append(f.docs, docObj)
	}

	f.setParents()

	return f, nil
}

func loadFileAndParents(fsys *fsys.FS, path string, child *file) ([]*file, error) {
	return loadFileAndParentsInt(fsys, path, child, []string{})
}

func loadFileAndParentsInt(fsys *fsys.FS, path string, child *file, stack []string) ([]*file, error) {
	if slices.Contains(stack, path) {
		return nil, fmt.Errorf("%s: %w", strings.Join(append(stack, path), " -> "), ErrCircularRef)
	}

	f, err := loadFile(fsys, path, child)
	if err != nil {
		return nil, err
	}

	stack = append(stack, path)

	parents, err := f.parents(fsys)
	if err != nil {
		return nil, err
	}

	files := []*file{}

	for _, parent := range parents {
		// If the current file has no docs, skip it in the hierarchy
		child2 := f
		if len(f.docs) == 0 {
			child2 = child
		}

		parentFiles, err := loadFileAndParentsInt(fsys, parent, child2, stack)
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
		doc.Parents = append(doc.Parents, f.docs...)
	}
}

func (f *file) parents(fsys *fsys.FS) ([]string, error) {
	parents, err := f.parentsFromDirective(fsys)
	if err != nil {
		return nil, err
	}

	if parents != nil {
		return parents, nil
	}

	return f.parentsFromFilename(fsys)
}

func (f *file) parentsFromDirective(fsys *fsys.FS) ([]string, error) {
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
			val3, err := utils.ToStringList(val2)
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

func (f *file) parentsFromFilename(fsys *fsys.FS) ([]string, error) {
	if utils.IsStdin(f.path) {
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

		extPath := fsys.FindFile(layerPath)
		if extPath == "" {
			return nil, fmt.Errorf("[%s]: %w", layerPath, ErrMissingFile)
		}

		return []string{extPath}, nil
	}
}

func (f *file) toAbsolutePaths(fsys *fsys.FS, paths []string) ([]string, error) {
	ret := []string{}

	for _, path := range paths {
		path = filepath.Join(filepath.Dir(f.path), path)

		matches, err := fsys.GlobFiles(path)
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
