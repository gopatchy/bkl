package format

import (
	"bytes"
	"regexp"

	"github.com/pelletier/go-toml/v2"
)

func tomlMarshalStream(vs []any) ([]byte, error) {
	first := true
	buf := &bytes.Buffer{}
	enc := toml.NewEncoder(buf)

	for _, v := range vs {
		first2 := first
		first = false

		if !first2 {
			buf.Write([]byte("---\n"))
		}

		err := enc.Encode(v)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

var tomlRE = regexp.MustCompile(`(?m)^(\+\+\+|---)$`)

func tomlUnmarshalStream(in []byte) ([]any, error) {
	parts := tomlRE.Split(string(in), -1)
	ret := []any{}

	for _, s := range parts {
		var obj any

		err := toml.Unmarshal([]byte(s), &obj)
		if err != nil {
			return nil, err
		}

		ret = append(ret, obj)
	}

	return ret, nil
}
