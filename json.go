package bkl

import (
	"bytes"
	"encoding/json"
)

func decodeJSON(in []byte) (any, error) {
	var obj any

	err := json.Unmarshal(in, &obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func encodeJSON(obj any) ([]byte, error) {
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return append(out, '\n'), nil
}

func encodeJSONPretty(obj any) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")

	err := enc.Encode(obj)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
