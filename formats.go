package bkl

import (
	"encoding/json"
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type format struct {
	marshal   func(any) ([]byte, error)
	unmarshal func([]byte, any) error
}

var formatByExtension = map[string]format{
	"json": {
		marshal:   jsonMarshal,
		unmarshal: json.Unmarshal,
	},
	"json-pretty": {
		marshal:   jsonMarshalPretty,
		unmarshal: json.Unmarshal,
	},
	"toml": {
		marshal:   toml.Marshal,
		unmarshal: toml.Unmarshal,
	},
	"yaml": {
		marshal:   yaml.Marshal,
		unmarshal: yaml.Unmarshal,
	},
}

func getFormat(name string) (*format, error) {
	f, found := formatByExtension[name]
	if !found {
		return nil, fmt.Errorf("%s: %w", name, ErrUnknownFormat)
	}

	return &f, nil
}

func (f *format) encode(v any) ([]byte, error) {
	return f.marshal(v)
}

func (f *format) decode(in []byte) (any, error) {
	var obj any

	err := f.unmarshal(in, &obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}
