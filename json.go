package bkl

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func jsonMarshalStream(vs []any) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)

	for _, v := range vs {
		err := enc.Encode(v)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func jsonMarshalStreamPretty(vs []any) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")

	for _, v := range vs {
		err := enc.Encode(v)
		if err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func jsonUnmarshalStream(in []byte) ([]any, error) {
	dec := json.NewDecoder(bytes.NewReader(in))
	ret := []any{}

	for {
		var obj any

		err := dec.Decode(&obj)
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}

		ret = append(ret, obj)
	}

	return ret, nil
}
