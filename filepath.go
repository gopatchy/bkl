package bkl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Ext returns the file extension for path, or "".
//
// It differs from [filepath.Ext] by not including the leading `.`.
func Ext(path string) string {
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
