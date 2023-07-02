package bkl

import "encoding/json"

func decodeJSON(in []byte) (any, error) {
	var obj any

	err := json.Unmarshal(in, obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func encodeJSON(obj any) ([]byte, error) {
	return json.Marshal(obj)
}
