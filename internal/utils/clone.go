package utils

import "gopkg.in/yaml.v3"

func DeepClone(v any) (any, error) {
	yml, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}

	var ret any

	err = yaml.Unmarshal(yml, &ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
