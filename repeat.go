package bkl

import (
	"fmt"
)

func repeat(doc *Document) ([]*Document, error) {
	switch obj := doc.Data.(type) {
	case map[string]any:
		return repeatMap(doc, obj)

	case []any:
		return repeatList(doc, obj)

	default:
		return []*Document{doc}, nil
	}
}

func repeatMap(doc *Document, data map[string]any) ([]*Document, error) {
	if found, v, data := popMapValue(data, "$repeat"); found {
		doc.Data = data
		return repeatDoc(doc, v)
	}

	return []*Document{doc}, nil
}

func repeatList(doc *Document, data []any) ([]*Document, error) {
	v, data2, err := popListMapValue(data, "$repeat")
	if err != nil {
		return nil, err
	}

	if v != nil {
		doc.Data = data2
		return repeatDoc(doc, v)
	}

	return []*Document{doc}, nil
}

func repeatDoc(doc *Document, v any) ([]*Document, error) {
	switch v2 := v.(type) {
	case int:
		return repeatFromInt(doc, "$repeat", v2, map[string]any{})

	case map[string]any:
		return repeatFromMap(doc, v2)

	default:
		return nil, fmt.Errorf("$repeat: %T (%w)", v, ErrInvalidRepeat)
	}
}

func repeatFromInt(doc *Document, name string, count int, vars map[string]any) ([]*Document, error) {
	ret := []*Document{}

	for i := 0; i < count; i++ {
		doc2, err := doc.Clone(fmt.Sprintf("%s=%d", name, i))
		if err != nil {
			return nil, err
		}

		doc2.Vars[name] = i

		for k, v := range vars {
			doc2.Vars[k] = v
		}

		ret = append(ret, doc2)
	}

	return ret, nil
}

func repeatFromMap(doc *Document, rs map[string]any) ([]*Document, error) {
	docs := []*Document{doc}

	vars := map[string]any{}
	for k, v := range rs {
		vars[fmt.Sprintf("$repeat.%s", k)] = v
	}

	for name, count := range sortedMap(rs) {
		count2, ok := count.(int)
		if !ok {
			return nil, fmt.Errorf("%T (%w)", count, ErrInvalidRepeat)
		}

		tmp := []*Document{}

		for _, d := range docs {
			ds, err := repeatFromInt(d, fmt.Sprintf("$repeat:%s", name), count2, vars)
			if err != nil {
				return nil, err
			}

			tmp = append(tmp, ds...)
		}

		docs = tmp
	}

	return docs, nil
}
