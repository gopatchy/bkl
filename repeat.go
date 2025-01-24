package bkl

import (
	"fmt"
)

func repeatDoc(doc *Document) ([]*Document, error) {
	switch obj := doc.Data.(type) {
	case map[string]any:
		return repeatDocMap(doc, obj)

	case []any:
		return repeatDocList(doc, obj)

	default:
		return []*Document{doc}, nil
	}
}

func repeatDocMap(doc *Document, data map[string]any) ([]*Document, error) {
	if found, v, data := popMapValue(data, "$repeat"); found {
		doc.Data = data
		return repeatDocGen(doc, v)
	}

	return []*Document{doc}, nil
}

func repeatDocList(doc *Document, data []any) ([]*Document, error) {
	v, data2, err := popListMapValue(data, "$repeat")
	if err != nil {
		return nil, err
	}

	if v != nil {
		doc.Data = data2
		return repeatDocGen(doc, v)
	}

	return []*Document{doc}, nil
}

func repeatDocGen(doc *Document, v any) ([]*Document, error) {
	switch v2 := v.(type) {
	case int:
		return repeatDocGenFromInt(doc, "$repeat", v2, map[string]any{})

	case map[string]any:
		return repeatDocGenFromMap(doc, v2)

	default:
		return nil, fmt.Errorf("$repeat: %T (%w)", v, ErrInvalidRepeat)
	}
}

func repeatDocGenFromInt(doc *Document, name string, count int, vars map[string]any) ([]*Document, error) {
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

func repeatDocGenFromMap(doc *Document, rs map[string]any) ([]*Document, error) {
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
			ds, err := repeatDocGenFromInt(d, fmt.Sprintf("$repeat:%s", name), count2, vars)
			if err != nil {
				return nil, err
			}

			tmp = append(tmp, ds...)
		}

		docs = tmp
	}

	return docs, nil
}
