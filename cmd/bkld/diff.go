package main

import (
	"maps"
	"reflect"
	"slices"

	"github.com/gopatchy/bkl"
)

func diffDoc(dst, src *bkl.Document) (any, error) {
	doc, err := diff(dst.Data, src.Data)
	if err != nil {
		fatal(err)
	}

	switch doc2 := doc.(type) {
	case map[string]any:
		doc2["$match"] = map[string]any{}
		return doc2, nil

	case []any:
		doc2 = append([]any{
			map[string]any{
				"$match": map[string]any{},
			},
		}, doc2...)
		return doc2, nil

	default:
		return doc, nil
	}
}

func diff(dst, src any) (any, error) {
	switch dst2 := dst.(type) {
	case map[string]any:
		return diffMap(dst2, src)

	case []any:
		return diffList(dst2, src)

	default:
		if dst2 == src {
			return nil, nil
		}

		return dst2, nil
	}
}

func diffMap(dst map[string]any, src any) (any, error) {
	switch src2 := src.(type) {
	case map[string]any:
		return diffMapMap(dst, src2)

	default:
		// Different types
		return dst, nil
	}
}

func diffMapMap(dst, src map[string]any) (any, error) {
	ret := map[string]any{}

	for k, v := range dst {
		v2, found := src[k]
		if !found {
			ret[k] = v
			continue
		}

		v3, err := diff(v, v2)
		if err != nil {
			return nil, err
		}

		if v3 != nil {
			ret[k] = v3
		}
	}

	for k := range src {
		_, found := dst[k]
		if found {
			continue
		}

		ret[k] = "$delete"
	}

	if len(ret) == 0 {
		return nil, nil
	}

	return ret, nil
}

func diffList(dst []any, src any) (any, error) {
	switch src2 := src.(type) {
	case []any:
		return diffListList(dst, src2)

	default:
		return dst, nil
	}
}

func diffListList(dst, src []any) (any, error) { //nolint:unparam
	ret := []any{}

outer1:
	for _, v1 := range dst {
		for _, v2 := range src {
			if reflect.DeepEqual(v1, v2) {
				continue outer1
			}
		}

		ret = append(ret, v1)
	}

outer2:
	for _, v1 := range src {
		for _, v2 := range dst {
			if reflect.DeepEqual(v1, v2) {
				continue outer2
			}
		}

		v1Map, ok := v1.(map[string]any)
		if ok {
			del := map[string]any{
				"$delete": maps.Clone(v1Map),
			}
			ret = append(ret, del)
		} else {
			// Give up patching individual entries, replace the whole list
			dst = slices.Clone(dst)
			dst = append(dst, map[string]any{"$replace": true})
			return dst, nil
		}
	}

	if len(ret) == 0 {
		return nil, nil
	}

	return ret, nil
}
