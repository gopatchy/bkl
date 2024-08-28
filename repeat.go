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
		return repeatFromInt(doc, "$repeat", v2)

	case []any:
		return repeatFromList(doc, v2)

	default:
		return nil, fmt.Errorf("$repeat: %T (%w)", v, ErrInvalidRepeat)
	}
}

func repeatFromInt(doc *Document, name string, count int) ([]*Document, error) {
	ret := []*Document{}

	for i := 0; i < count; i++ {
		doc2, err := doc.Clone(fmt.Sprintf("%s=%d", name, i))
		if err != nil {
			return nil, err
		}

		doc2.Vars[name] = i
		ret = append(ret, doc2)
	}

	return ret, nil
}

func repeatFromList(doc *Document, rs []any) ([]*Document, error) {
	docs := []*Document{doc}

	for _, r := range rs {
		r2, ok := r.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%T (%w)", 2, ErrInvalidRepeat)
		}

		var found bool
		var v any

		var count int
		if found, v, r2 = popMapValue(r2, "$count"); found {
			count, ok = v.(int)
			if !ok {
				return nil, fmt.Errorf("%T (%w)", v, ErrInvalidRepeatCount)
			}
		} else {
			return nil, fmt.Errorf("%#v (%w)", r, ErrMissingRepeatCount)
		}

		var name string
		if found, v, r2 = popMapValue(r2, "$name"); found {
			name, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("%T (%w)", v, ErrInvalidRepeatName)
			}
		} else {
			return nil, fmt.Errorf("%#v (%w)", r, ErrMissingRepeatName)
		}

		tmp := []*Document{}

		for _, d := range docs {
			ds, err := repeatFromInt(d, name, count)
			if err != nil {
				return nil, err
			}

			tmp = append(tmp, ds...)
		}

		docs = tmp
	}

	return docs, nil
}
