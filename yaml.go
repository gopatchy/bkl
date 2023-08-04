package bkl

import (
	"bytes"
	"regexp"

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

var yamlRE = regexp.MustCompile("(?m)^---$")

func yamlUnmarshalStream(in []byte) ([]any, error) {
	// Differs from repeated yaml.Decode by treating "---\n---" as an empty
	// document, rather than skipping it.

	parts := yamlRE.Split(string(in), -1)
	ret := []any{}

	for _, s := range parts {
		var obj any

		err := yaml.Unmarshal([]byte(s), &obj)
		if err != nil {
			return nil, err
		}

		ret = append(ret, obj)
	}

	return ret, nil
}
