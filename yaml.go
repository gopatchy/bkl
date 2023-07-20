package bkl

import (
	"bytes"

	"gopkg.in/yaml.v3"
)

func yamlMarshal(v any) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)

	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
