package bkl

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/gopatchy/bkl/polyfill"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

type Format struct {
	marshal   func(any) ([]byte, error)
	unmarshal func([]byte, any) error
}

var formatByExtension = map[string]Format{
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
		marshal:   yamlMarshal,
		unmarshal: yaml.Unmarshal,
	},
}

func GetFormat(name string) (*Format, error) {
	f, found := formatByExtension[name]
	if !found {
		return nil, fmt.Errorf("%s: %w", name, ErrUnknownFormat)
	}

	return &f, nil
}

func (f *Format) Marshal(v any) ([]byte, error) {
	if v == nil {
		return []byte{}, nil
	}

	ret, err := f.marshal(v)
	if err != nil {
		return nil, polyfill.ErrorsJoin(err, ErrMarshal)
	}

	return ret, nil
}

func (f *Format) MarshalStream(vs []any) ([]byte, error) {
	bs := [][]byte{}

	for _, v := range vs {
		b, err := f.Marshal(v)
		if err != nil {
			return nil, err
		}

		bs = append(bs, b)
	}

	return bytes.Join(bs, []byte("---\n")), nil
}

func (f *Format) Unmarshal(in []byte) (any, error) {
	var obj any

	err := f.unmarshal(in, &obj)
	if err != nil {
		return nil, polyfill.ErrorsJoin(err, ErrUnmarshal)
	}

	return obj, nil
}
