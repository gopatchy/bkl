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
		return repeatInt(doc, v2)

	default:
		return nil, fmt.Errorf("$repeat: %T (%w)", v, ErrInvalidRepeat)
	}
}

func repeatInt(doc *Document, count int) ([]*Document, error) {
	ret := []*Document{}

	for i := 0; i < count; i++ {
		doc2, err := doc.Clone(fmt.Sprintf("repeat%d", i))
		if err != nil {
			return nil, err
		}

		doc2.Vars["$repeat"] = i

		doc2.Data, err = constEval(doc2.Data, doc2)
		if err != nil {
			return nil, err
		}

		ret = append(ret, doc2)
	}

	return ret, nil
}
