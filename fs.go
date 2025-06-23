package bkl

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

type FS struct {
	fsys fs.FS
}

func NewFS(fsys fs.FS) *FS {
	return &FS{
		fsys: fsys,
	}
}

func (f *FS) Open(name string) (fs.File, error) {
	return f.fsys.Open(f.convertToFS(name))
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	rdf := f.fsys.(fs.ReadDirFS)
	return rdf.ReadDir(f.convertToFS(name))
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
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

func (f *FS) Glob(pattern string) ([]string, error) {
	dir, file := filepath.Split(pattern)
	dir = strings.TrimSuffix(dir, "/")

	entries, err := f.ReadDir(dir)
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

func (f *FS) findFile(path string) string {
	for ext := range formatByExtension {
		extPath := fmt.Sprintf("%s.%s", path, ext)
		if _, err := f.Stat(extPath); err == nil {
			return extPath
		}
	}
	return ""
}

func (f *FS) globFiles(path string) ([]string, error) {
	pat := fmt.Sprintf("%s.*", path)
	matches, err := f.Glob(pat)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pat, err)
	}

	ret := []string{}

	for _, match := range matches {
		if _, found := formatByExtension[Ext(match)]; !found {
			continue
		}

		ret = append(ret, match)
	}

	return ret, nil
}

func Ext(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}
