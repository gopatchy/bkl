package bkl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.jetpack.io/typeid"
)

type file struct {
	id    typeid.TypeID
	child *file
	path  string
	docs  []any
}

func (p *Parser) loadFile(path string, child *file) (*file, error) {
	f := &file{
		id:    typeid.Must(typeid.New("file")),
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

func (p *Parser) loadFileAndParents(path string, child *file) ([]*file, error) {
	f, err := p.loadFile(path, child)
	if err != nil {
		return nil, err
	}

	parents, err := f.parents()
	if err != nil {
		return nil, err
	}

	files := []*file{}

	for _, parent := range parents {
		parentFiles, err := p.loadFileAndParents(parent, f)
		if err != nil {
			return nil, err
		}

		files = append(files, parentFiles...)
	}

	return append(files, f), nil
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

	for i, doc := range f.docs {
		docMap, ok := doc.(map[string]any)
		if !ok {
			continue
		}

		var found bool
		var val any
		found, val, docMap = popMapValue(docMap, "$parent")
		f.docs[i] = docMap

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

func (f *file) toAbsolutePaths(paths []string) ([]string, error) {
	ret := []string{}

	for _, path := range paths {
		path = filepath.Join(filepath.Dir(f.path), path)

		path2 := findFile(path)
		if path2 == "" {
			return nil, fmt.Errorf("%s: %w", path, ErrMissingFile)
		}

		ret = append(ret, path2)
	}

	return ret, nil
}

func (f *file) String() string {
	return f.path
}

func isStdin(path string) bool {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) == "-"
}
