package format

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/magiconair/properties"
)

func propertiesMarshalStream(stream []any) ([]byte, error) {
	if len(stream) != 1 {
		return nil, fmt.Errorf("properties format only supports single document")
	}

	obj, ok := stream[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("properties format requires top-level map, got %T", stream[0])
	}

	p := properties.NewProperties()

	if err := flattenMap("", obj, p); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := p.Write(&buf, properties.UTF8); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func flattenMap(prefix string, m map[string]any, p *properties.Properties) error {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := m[key]
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case string:
			p.Set(fullKey, v)
		case bool:
			p.Set(fullKey, fmt.Sprintf("%t", v))
		case int, int64, float64:
			p.Set(fullKey, fmt.Sprintf("%v", v))
		case map[string]any:
			if err := flattenMap(fullKey, v, p); err != nil {
				return err
			}
		case []any:
			var values []string
			for _, item := range v {
				values = append(values, fmt.Sprintf("%v", item))
			}
			p.Set(fullKey, strings.Join(values, ","))
		default:
			p.Set(fullKey, fmt.Sprintf("%v", v))
		}
	}
	return nil
}

func propertiesUnmarshalStream(data []byte) ([]any, error) {
	p, err := properties.Load(data, properties.UTF8)
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)
	for _, key := range p.Keys() {
		value := p.GetString(key, "")
		setNestedValue(result, key, value)
	}

	return []any{result}, nil
}

func setNestedValue(m map[string]any, key string, value string) {
	parts := strings.Split(key, ".")
	current := m

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
		} else {
			if _, exists := current[part]; !exists {
				current[part] = make(map[string]any)
			}
			if nextMap, ok := current[part].(map[string]any); ok {
				current = nextMap
			} else {
				return
			}
		}
	}
}
