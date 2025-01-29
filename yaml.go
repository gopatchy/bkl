package bkl

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"

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
		var node yaml.Node

		err := yaml.Unmarshal([]byte(s), &node)
		if err != nil {
			return nil, err
		}

		obj, err := yamlTranslateNode(&node)
		if err != nil {
			return nil, err
		}

		ret = append(ret, obj)
	}

	return ret, nil
}

func yamlTranslateNode(node *yaml.Node) (any, error) {
	switch node.Kind {
	case yaml.DocumentNode:
		return yamlTranslateNode(node.Content[0])

	case yaml.SequenceNode:
		ret := []any{}

		for _, v := range node.Content {
			v2, err := yamlTranslateNode(v)
			if err != nil {
				return nil, err
			}

			ret = append(ret, v2)
		}

		return ret, nil

	case yaml.MappingNode:
		ret := map[string]any{}

		for i := 0; i+1 < len(node.Content); i += 2 {
			v2, err := yamlTranslateNode(node.Content[i+1])
			if err != nil {
				return nil, err
			}

			ret[node.Content[i].Value] = v2
		}

		return ret, nil

	case yaml.ScalarNode:
		switch node.ShortTag() {
		case "!!bool":
			return strconv.ParseBool(node.Value)

		case "!!int":
			v, err := strconv.ParseInt(node.Value, 10, 32)
			if err == nil {
				return int(v), nil
			}

			return strconv.ParseInt(node.Value, 10, 64)

		case "!!float":
			v, err := strconv.ParseFloat(node.Value, 32)
			if err == nil {
				return v, nil
			}

			return strconv.ParseFloat(node.Value, 64)

		case "!!null":
			return nil, nil

		case "!!str", "!!timestamp":
			return node.Value, nil

		default:
			return nil, fmt.Errorf("unknown yaml short tag: %s (%w)", node.ShortTag(), ErrInvalidType)
		}

	case yaml.AliasNode:
		return yamlTranslateNode(node.Alias)

	case 0:
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown yaml type: %d (%w)", node.Kind, ErrInvalidType)
	}
}
