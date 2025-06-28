package fsys

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/gopatchy/bkl/internal/format"
	"github.com/gopatchy/bkl/internal/utils"
)

type FS struct {
	fsys fs.FS
}

func New(fsys fs.FS) *FS {
	return &FS{
		fsys: fsys,
	}
}

func (f *FS) Open(name string) (fs.File, error) {
	return f.fsys.Open(f.convertToFS(name))
}

func (f *FS) readDir(name string) ([]fs.DirEntry, error) {
	rdf := f.fsys.(fs.ReadDirFS)
	return rdf.ReadDir(f.convertToFS(name))
}

func (f *FS) stat(name string) (fs.FileInfo, error) {
	sf, ok := f.fsys.(fs.StatFS)
	if ok {
		return sf.Stat(f.convertToFS(name))
	}

	// Fallback: use Open and get FileInfo from the file
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return file.Stat()
}

func (f *FS) glob(pattern string) ([]string, error) {
	dir, file := filepath.Split(pattern)
	dir = strings.TrimSuffix(dir, "/")

	entries, err := f.readDir(dir)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	var matches []string
	for _, entry := range entries {
		matched, err := filepath.Match(file, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", pattern, err)
		}
		if matched {
			fullPath := filepath.Join(dir, entry.Name())
			matches = append(matches, fullPath)
		}
	}

	return matches, nil
}

func (f *FS) convertToFS(path string) string {
	result := strings.TrimPrefix(path, "/")
	if result == "" {
		return "."
	}
	return result
}

func (f *FS) FindFile(path string) string {
	for _, ext := range format.Extensions() {
		extPath := fmt.Sprintf("%s.%s", path, ext)
		if _, err := f.stat(extPath); err == nil {
			return extPath
		}
	}
	return ""
}

func (f *FS) GlobFiles(path string) ([]string, error) {
	pat := fmt.Sprintf("%s.*", path)
	matches, err := f.glob(pat)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pat, err)
	}

	ret := []string{}

	for _, match := range matches {
		if _, err := format.Get(utils.Ext(match)); err != nil {
			continue
		}

		ret = append(ret, match)
	}

	return ret, nil
}
