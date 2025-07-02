package pathutil

import (
	"fmt"
	"strings"

	"github.com/gopatchy/bkl/pkg/errors"
)

// Get retrieves a value from a nested structure using a slice of path parts.
// It returns an error if the path is not found or cannot be traversed.
func Get(data any, parts []string) (any, error) {
	if len(parts) == 0 {
		return data, nil
	}

	switch obj := data.(type) {
	case map[string]any:
		val, found := obj[parts[0]]
		if !found {
			return nil, fmt.Errorf("%v: %w", parts, errors.ErrRefNotFound)
		}
		return Get(val, parts[1:])
	default:
		return nil, fmt.Errorf("%v: %w", parts, errors.ErrRefNotFound)
	}
}

// GetString retrieves a value from a nested structure using a dot-separated path string
// and converts it to a string. Returns an error if the path is not found or cannot be traversed.
func GetString(data any, path string) (string, error) {
	if path == "" {
		return "", nil
	}

	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		switch obj := current.(type) {
		case map[string]any:
			val, found := obj[part]
			if !found {
				return "", fmt.Errorf("key %q not found in path %q: %w", part, path, errors.ErrRefNotFound)
			}
			current = val
		default:
			return "", fmt.Errorf("cannot traverse %q in non-map type %T: %w", part, current, errors.ErrRefNotFound)
		}
	}

	return fmt.Sprint(current), nil
}

// Set sets a value at a path in a nested map structure.
// It creates intermediate maps as needed.
func Set(data map[string]any, parts []string, value any) {
	if len(parts) == 0 {
		return
	}

	if len(parts) == 1 {
		data[parts[0]] = value
		return
	}

	if _, exists := data[parts[0]]; !exists {
		data[parts[0]] = map[string]any{}
	}

	if next, ok := data[parts[0]].(map[string]any); ok {
		Set(next, parts[1:], value)
	}
}

// GetNoError retrieves a value without returning an error.
// This is useful for cases where missing paths are expected.
func GetNoError(data any, parts []string) (any, error) {
	if len(parts) == 0 {
		return data, nil
	}

	switch obj := data.(type) {
	case map[string]any:
		val, found := obj[parts[0]]
		if !found {
			return nil, fmt.Errorf("path not found: %v", parts[0])
		}
		return GetNoError(val, parts[1:])
	default:
		return nil, fmt.Errorf("cannot traverse path in %T", data)
	}
}

// SplitPath splits a dot-separated path string into parts.
func SplitPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}
