package bkl

import (
	"bytes"
	"encoding/json"
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
