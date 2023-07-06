package bkl

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func getParent(path string) (*string, error) {
	parent, err := getParentFromDirective(path)
	if err != nil {
		return nil, err
	}

	if parent != nil {
		return parent, nil
	}

	parent, err = getParentFromSymlink(path)
	if err != nil {
		return nil, err
	}

	if parent != nil {
		return parent, nil
	}

	return getParentFromFilename(path)
}

func getParentFromDirective(path string) (*string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	defer fh.Close()

	b, err := io.ReadAll(fh)
	if err != nil {
		return nil, err
	}

	ext := Ext(path)

	f, found := formatByExtension[ext]
	if !found {
		return nil, fmt.Errorf("%s: %w", ext, ErrUnknownFormat)
	}

	patch, err := f.decode(b)
	if err != nil {
		return nil, fmt.Errorf("%w / %w", err, ErrDecode)
	}

	patchMap, ok := patch.(map[string]any)
	if !ok {
		return nil, nil
	}

	if parent, found := patchMap["$parent"]; found {
		if parent == nil {
			return &baseTemplate, nil
		}

		parentStr, ok := parent.(string)
		if !ok {
			return nil, fmt.Errorf("%T: %w", parent, ErrInvalidParentType)
		}

		parentPath := findFile(parentStr)
		if parentPath == "" {
			return nil, fmt.Errorf("%s: %w", parentStr, ErrMissingFile)
		}

		return &parentPath, nil
	}

	return nil, nil
}

func getParentFromSymlink(path string) (*string, error) {
	dest, _ := os.Readlink(path)

	if dest == "" {
		// Not a link
		return nil, nil
	}

	return getParentFromFilename(dest)
}

func getParentFromFilename(path string) (*string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	parts := strings.Split(base, ".")
	// Last part is file extension

	switch {
	case len(parts) < 2:
		return nil, fmt.Errorf("%s: %w", path, ErrInvalidFilename)

	case len(parts) == 2:
		return &baseTemplate, nil

	default:
		layerPath := filepath.Join(dir, strings.Join(parts[:len(parts)-2], "."))

		extPath := findFile(layerPath)
		if extPath == "" {
			return nil, fmt.Errorf("%s: %w", layerPath, ErrMissingFile)
		}

		return &extPath, nil
	}
}
