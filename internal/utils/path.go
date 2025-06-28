package utils

import (
	"path/filepath"
	"strings"
)

func Ext(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

func IsStdin(path string) bool {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) == "-"
}
