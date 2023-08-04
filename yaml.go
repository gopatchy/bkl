package bkl

import (
	"bytes"
	"regexp"

	"gopkg.in/yaml.v3"
)

func yamlMarshalStream(vs []any) ([]byte, error) {
	// Differs from repeated yaml.Encode by writing "---\n---" for an empty
	// document rather than "null".

	first := true
	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)

	for _, v := range vs {
		first2 := first
		first = false

		if v == nil {
			if !first2 {
				buf.Write([]byte("---\n"))
			}

			continue
		}

		err := enc.Encode(v)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

var yamlRE = regexp.MustCompile(`(?m)^---$`)

func yamlUnmarshalStream(in []byte) ([]any, error) {
	// Differs from repeated yaml.Decode by treating "---\n---" as an empty
	// document rather than skipping it.

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
