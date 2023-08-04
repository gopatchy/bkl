package bkl

import (
	"bytes"
	"fmt"

	"github.com/gopatchy/bkl/polyfill"
	"github.com/pelletier/go-toml/v2"
)

type Format struct {
	marshal         func(any) ([]byte, error)
	marshalStream   func([]any) ([]byte, error)
	unmarshal       func([]byte, any) error
	unmarshalStream func([]byte) ([]any, error)
	delimiter       string
}

var formatByExtension = map[string]Format{
	"json": {
		marshalStream:   jsonMarshalStream,
		unmarshalStream: jsonUnmarshalStream,
	},
	"jsonl": {
		marshalStream:   jsonMarshalStream,
		unmarshalStream: jsonUnmarshalStream,
	},
	"json-pretty": {
		marshal:         jsonMarshalPretty,
		unmarshalStream: jsonUnmarshalStream,
	},
	"toml": {
		marshal:   toml.Marshal,
		unmarshal: toml.Unmarshal,
		delimiter: "---\n",
	},
	"yaml": {
		marshalStream:   yamlMarshalStream,
		unmarshalStream: yamlUnmarshalStream,
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
	if f.marshalStream != nil {
		return f.marshalStream(vs)
	}

	bs := [][]byte{}

	for _, v := range vs {
		b, err := f.Marshal(v)
		if err != nil {
			return nil, err
		}

		bs = append(bs, b)
	}

	return bytes.Join(bs, []byte(f.delimiter)), nil
}

func (f *Format) Unmarshal(in []byte) (any, error) {
	var obj any

	err := f.unmarshal(in, &obj)
	if err != nil {
		return nil, polyfill.ErrorsJoin(err, ErrUnmarshal)
	}

	return obj, nil
}

func (f *Format) UnmarshalStream(in []byte) ([]any, error) {
	if f.unmarshalStream != nil {
		return f.unmarshalStream(in)
	}

	ret := []any{}

	for i, raw := range splitStream(in) {
		doc, err := f.Unmarshal(raw)
		if err != nil {
			return nil, fmt.Errorf("[doc%d]: %w", i, err)
		}

		ret = append(ret, doc)
	}

	return ret, nil
}

func splitStream(in []byte) [][]byte {
	ret := [][]byte{}

	for {
		if bytes.HasPrefix(in, []byte("---\n")) {
			ret = append(ret, []byte(""))
			in = bytes.TrimPrefix(in, []byte("---\n"))

			continue
		}

		parts := bytes.SplitN(in, []byte("\n---\n"), 2)
		if len(parts) == 1 {
			ret = append(ret, in)
			break
		}

		ret = append(ret, append(parts[0], '\n'))
		in = parts[1]
	}

	return ret
}
