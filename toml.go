package bkl

import (
	"github.com/pelletier/go-toml/v2"
)

func decodeTOML(in []byte) (any, error) {
	var obj any

	err := toml.Unmarshal(in, &obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func encodeTOML(obj any) ([]byte, error) {
	out, err := toml.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return out, nil
}
