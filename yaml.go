package bkl

import "gopkg.in/yaml.v3"

func decodeYAML(in []byte) (any, error) {
	var obj any

	err := yaml.Unmarshal(in, &obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func encodeYAML(obj any) ([]byte, error) {
	return yaml.Marshal(obj)
}
