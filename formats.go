package bkl

import (
	"fmt"
)

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

func getFormat(name string) (*Format, error) {
	f, found := formatByExtension[name]
	if !found {
		return nil, fmt.Errorf("%s: %w", name, ErrUnknownFormat)
	}

	return &f, nil
}
