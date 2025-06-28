package file

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
	"github.com/gopatchy/bkl/internal/normalize"
	"github.com/gopatchy/bkl/internal/utils"
	"github.com/gopatchy/bkl/pkg/errors"
)

type File struct {
	ID    string
	Child *File
	Path  string
	Docs  []*document.Document
}

func Load(fsys *fsys.FS, path string, child *File) (*File, error) {
	f := &File{
		ID:    path,
		Child: child,
		Path:  path,
	}

	if child != nil {
		f.ID = fmt.Sprintf("%s|%s", child.ID, f.ID)
	}

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

		doc, err = normalize.Document(doc)
		if err != nil {
			return nil, fmt.Errorf("[%s]: %w", id, err)
		}

		docObj := document.NewWithData(id, doc)
		f.Docs = append(f.Docs, docObj)
	}

	f.setParents()

	return f, nil
}

func LoadAndParents(fsys *fsys.FS, path string, child *File) ([]*File, error) {
	return loadFileAndParentsInt(fsys, path, child, []string{})
}

func loadFileAndParentsInt(fsys *fsys.FS, path string, child *File, stack []string) ([]*File, error) {
	if slices.Contains(stack, path) {
		return nil, fmt.Errorf("%s: %w", strings.Join(append(stack, path), " -> "), errors.ErrCircularRef)
	}

	f, err := Load(fsys, path, child)
	if err != nil {
		return nil, err
	}

	stack = append(stack, path)

	parents, err := f.parents(fsys)
	if err != nil {
		return nil, err
	}

	files := []*File{}

	for _, parent := range parents {
		// If the current file has no docs, skip it in the hierarchy
		child2 := f
		if len(f.Docs) == 0 {
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

func (f *File) setParents() {
	if f.Child == nil {
		return
	}

	for _, doc := range f.Child.Docs {
		doc.Parents = append(doc.Parents, f.Docs...)
	}
}

func (f *File) parents(fsys *fsys.FS) ([]string, error) {
	parents, err := f.parentsFromDirective(fsys)
	if err != nil {
		return nil, err
	}

	if parents != nil {
		return parents, nil
	}

	return f.parentsFromFilename(fsys)
}

func (f *File) parentsFromDirective(fsys *fsys.FS) ([]string, error) {
	parents := []string{}
	noParent := false

	for _, doc := range f.Docs {
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
				return nil, fmt.Errorf("$parent=%#v: %w", val2, errors.ErrInvalidParent)
			}

			parents = append(parents, val3...)

		case bool:
			if val2 {
				return nil, fmt.Errorf("$parent=true: %w", errors.ErrInvalidParent)
			}

			noParent = true

		case nil:
			noParent = true
		}
	}

	if noParent {
		if len(parents) > 0 {
			return nil, fmt.Errorf("$parent=false and $parent=<string> in same file: %w", errors.ErrConflictingParent)
		}

		return []string{}, nil
	}

	if len(parents) == 0 {
		return nil, nil
	}

	return f.toAbsolutePaths(fsys, parents)
}

func (f *File) parentsFromFilename(fsys *fsys.FS) ([]string, error) {
	if utils.IsStdin(f.Path) {
		return []string{}, nil
	}

	dir := filepath.Dir(f.Path)
	base := filepath.Base(f.Path)

	parts := strings.Split(base, ".")
	// Last part is file extension

	switch {
	case len(parts) < 2:
		return nil, fmt.Errorf("[%s] %w", f.Path, errors.ErrInvalidFilename)

	case len(parts) == 2:
		return []string{}, nil

	default:
		layerPath := filepath.Join(dir, strings.Join(parts[:len(parts)-2], "."))

		extPath := fsys.FindFile(layerPath)
		if extPath == "" {
			return nil, fmt.Errorf("[%s]: %w", layerPath, errors.ErrMissingFile)
		}

		return []string{extPath}, nil
	}
}

func (f *File) toAbsolutePaths(fsys *fsys.FS, paths []string) ([]string, error) {
	ret := []string{}

	for _, path := range paths {
		path = filepath.Join(filepath.Dir(f.Path), path)

		matches, err := fsys.GlobFiles(path)
		if err != nil {
			return nil, err
		}

		if len(matches) == 0 {
			return nil, fmt.Errorf("%s: %w", path, errors.ErrMissingFile)
		}

		ret = append(ret, matches...)
	}

	return ret, nil
}

func (f *File) String() string {
	return f.ID
}
