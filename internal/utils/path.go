package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Ext(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

func IsStdin(path string) bool {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) == "-"
}

// MakePathsAbsolute converts relative paths to absolute paths using the provided working directory.
func MakePathsAbsolute(paths []string, workingDir string) ([]string, error) {
	result := make([]string, len(paths))
	for i, path := range paths {
		if filepath.IsAbs(path) {
			result[i] = path
		} else {
			result[i] = filepath.Join(workingDir, path)
		}
	}
	return result, nil
}

// RebasePathsToRoot rebases absolute paths to be relative to the root path.
func RebasePathsToRoot(absPaths []string, rootPath string, workingDir string) ([]string, error) {
	absRootPath := rootPath
	if !filepath.IsAbs(rootPath) {
		absRootPath = filepath.Join(workingDir, rootPath)
	}

	result := make([]string, len(absPaths))
	for i, path := range absPaths {
		relPath, err := filepath.Rel(absRootPath, path)
		if err != nil {
			return nil, fmt.Errorf("file %s outside root path: %w", path, err)
		}

		if strings.HasPrefix(relPath, "..") {
			return nil, fmt.Errorf("file %s outside root path", path)
		}

		result[i] = "/" + relPath
	}

	return result, nil
}

// PreparePathsForParser prepares paths by making them absolute and rebasing to root.
func PreparePathsForParser(paths []string, rootPath string, workingDir string) ([]string, error) {
	if workingDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workingDir = wd
	}

	absPaths, err := MakePathsAbsolute(paths, workingDir)
	if err != nil {
		return nil, err
	}

	return RebasePathsToRoot(absPaths, rootPath, workingDir)
}
