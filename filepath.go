package bkl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileMatch attempts to find a file with the same base name as path, but
// possibly with a different supported extension. It is intended to support
// "virtual" filenames that auto-convert from the format of the underlying
// real file.
//
// Returns the real filename and the requested output format, or
// ("", "", error).
func FileMatch(path string) (string, string, error) {
	f := ext(path)
	if _, found := formatByExtension[f]; !found {
		return "", "", fmt.Errorf("%s: %w", f, ErrInvalidType)
	}

	withoutExt := strings.TrimSuffix(path, "."+f)

	if filepath.Base(withoutExt) == "-" {
		return path, f, nil
	}

	realPath := findFile(withoutExt)

	if realPath == "" {
		return "", "", fmt.Errorf("%s.*: %w", withoutExt, ErrMissingFile)
	}

	return realPath, f, nil
}

func ext(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

func findFile(path string) string {
	for ext := range formatByExtension {
		extPath := fmt.Sprintf("%s.%s", path, ext)
		if _, err := os.Stat(extPath); errors.Is(err, os.ErrNotExist) {
			continue
		}

		return extPath
	}

	return ""
}
