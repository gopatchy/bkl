package format

import (
	"fmt"

	"github.com/gopatchy/bkl/pkg/errors"
)

// Format handles marshaling and unmarshaling for a specific file format
type Format struct {
	MarshalStream   func([]any) ([]byte, error)
	UnmarshalStream func([]byte) ([]any, error)
}

var formatByExtension = map[string]Format{
	"json": {
		MarshalStream:   jsonMarshalStream,
		UnmarshalStream: jsonUnmarshalStream,
	},
	"jsonl": {
		MarshalStream:   jsonMarshalStream,
		UnmarshalStream: jsonUnmarshalStream,
	},
	"json-pretty": {
		MarshalStream:   jsonMarshalStreamPretty,
		UnmarshalStream: jsonUnmarshalStream,
	},
	"toml": {
		MarshalStream:   tomlMarshalStream,
		UnmarshalStream: tomlUnmarshalStream,
	},
	"yaml": {
		MarshalStream:   yamlMarshalStream,
		UnmarshalStream: yamlUnmarshalStream,
	},
	"yml": {
		MarshalStream:   yamlMarshalStream,
		UnmarshalStream: yamlUnmarshalStream,
	},
}

// Get retrieves a format by name from the registry
func Get(name string) (*Format, error) {
	ft, found := formatByExtension[name]
	if !found {
		return nil, fmt.Errorf("%s: %w", name, errors.ErrUnknownFormat)
	}

	return &ft, nil
}

// Extensions returns all supported format extensions
func Extensions() []string {
	exts := make([]string, 0, len(formatByExtension))
	for ext := range formatByExtension {
		exts = append(exts, ext)
	}
	return exts
}
