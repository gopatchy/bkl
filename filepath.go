package bkl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
