package bkl

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

type FS struct {
	fsys fs.FS
	wd   string
}

func NewFS(fsys fs.FS, wd string) *FS {
	f := &FS{
		fsys: fsys,
		wd:   "/",
	}
	f.Chdir(wd)
	return f
}

func (f *FS) Abs(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Clean(filepath.Join(f.wd, name))
}

func (f *FS) Open(name string) (fs.File, error) {
	return f.fsys.Open(f.convertToFS(name))
}

func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	rdf := f.fsys.(fs.ReadDirFS)
	return rdf.ReadDir(f.convertToFS(name))
}

func (f *FS) Stat(name string) (fs.FileInfo, error) {
	sf := f.fsys.(fs.StatFS)
	return sf.Stat(f.convertToFS(name))
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
			if filepath.IsAbs(pattern) {
				matches = append(matches, fullPath)
			} else {
				rel, err := f.Rel(fullPath)
				if err != nil {
					return nil, fmt.Errorf("rel %s: %w", pattern, err)
				}

				matches = append(matches, rel)
			}
		}
	}

	return matches, nil
}

func (f *FS) Chdir(dir string) {
	f.wd = f.Abs(dir)
}

func (f *FS) Getwd() string {
	return f.wd
}

func (f *FS) Rel(target string) (string, error) {
	return filepath.Rel(f.wd, target)
}

func (f *FS) convertToFS(path string) string {
	return strings.TrimPrefix(f.Abs(path), "/")
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
		if _, found := formatByExtension[ext(match)]; !found {
			continue
		}

		ret = append(ret, match)
	}

	return ret, nil
}

func ext(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

