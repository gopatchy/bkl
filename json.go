package bkl

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func jsonMarshal(v any) ([]byte, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return append(out, '\n'), nil
}

func jsonMarshalPretty(v any) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")

	err := enc.Encode(v)
	if err != nil {
		return nil, err
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
