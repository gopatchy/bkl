package format

import (
	"bytes"
	"fmt"
	"io"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/gopatchy/bkl/pkg/errors"
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

func yamlUnmarshalStream(in []byte) ([]any, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(in))
	ret := []any{}

	for {
		var node yaml.Node

		err := decoder.Decode(&node)
		if err != nil {
			if err == io.EOF {
				break
			}
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

		// First see if there's a merge statement, and merge the referenced map(s) into ret.
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "<<" {
				v2, err := yamlTranslateNode(node.Content[i+1])
				if err != nil {
					return nil, err
				}

				err = yamlMerge(ret, v2, node.Content[i+1])
				if err != nil {
					return nil, err
				}
			}
		}

		// Next iterate over all the local values of the map.
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "<<" {
				continue
			}

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
			return nil, fmt.Errorf("unknown yaml short tag: %s (%w)", node.ShortTag(), errors.ErrInvalidType)
		}

	case yaml.AliasNode:
		return yamlTranslateNode(node.Alias)

	case 0:
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown yaml type: %d (%w)", node.Kind, errors.ErrInvalidType)
	}
}

// Merge mapping or list of mappings into a destination mapping, as per https://yaml.org/type/merge.html
func yamlMerge(dst map[string]any, src any, node *yaml.Node) error {
	switch src2 := src.(type) {
	case map[string]any:
		for k, v := range src2 {
			dst[k] = v
		}
	case []any:
		for i := len(src2) - 1; i >= 0; i-- {
			switch inner := src2[i].(type) {
			case map[string]any:
				for k, v := range inner {
					dst[k] = v
				}
			default:
				return fmt.Errorf("unknown type for merge target: %d (%w)", node.Kind, errors.ErrInvalidType)
			}
		}
	default:
		return fmt.Errorf("unknown type for merge target: %d (%w)", node.Kind, errors.ErrInvalidType)
	}

	return nil
}
