package bkl

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/gopatchy/bkl/internal/format"
	"github.com/gopatchy/bkl/internal/utils"
)

type fileSystem struct {
	fsys fs.FS
}

func newFS(fsys fs.FS) *fileSystem {
	return &fileSystem{
		fsys: fsys,
	}
}

func (f *fileSystem) open(name string) (fs.File, error) {
	return f.fsys.Open(f.convertToFS(name))
}

func (f *fileSystem) readDir(name string) ([]fs.DirEntry, error) {
	rdf := f.fsys.(fs.ReadDirFS)
	return rdf.ReadDir(f.convertToFS(name))
}

func (f *fileSystem) stat(name string) (fs.FileInfo, error) {
	sf, ok := f.fsys.(fs.StatFS)
	if ok {
		return sf.Stat(f.convertToFS(name))
	}

	// Fallback: use Open and get FileInfo from the file
	file, err := f.open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return file.Stat()
}

func (f *fileSystem) glob(pattern string) ([]string, error) {
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

func (f *fileSystem) convertToFS(path string) string {
	result := strings.TrimPrefix(path, "/")
	if result == "" {
		return "."
	}
	return result
}

func (f *fileSystem) findFile(path string) string {
	for _, ext := range format.Extensions() {
		extPath := fmt.Sprintf("%s.%s", path, ext)
		if _, err := f.stat(extPath); err == nil {
			return extPath
		}
	}
	return ""
}

func (f *fileSystem) globFiles(path string) ([]string, error) {
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
