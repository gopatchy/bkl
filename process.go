package bkl

import (
	"fmt"
	"strings"

	"github.com/gopatchy/bkl/polyfill"
)

func Process(obj, mergeFrom any, mergeFromDocs []any) (any, error) {
	return process(obj, mergeFrom, mergeFromDocs, 0)
}

// process() and descendants intentionally mutate obj to handle chained
// references
func process(obj, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	if depth > 1000 {
		return nil, fmt.Errorf("%#v: %w", obj, ErrCircularRef)
	}

	switch objType := obj.(type) {
	case map[string]any:
		return processMap(objType, mergeFrom, mergeFromDocs, depth+1)

	case []any:
		return processList(objType, mergeFrom, mergeFromDocs, depth+1)

	case string:
		return processString(objType, mergeFrom, mergeFromDocs, depth+1)

	default:
		return obj, nil
	}
}

func processMap(obj map[string]any, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	m := obj["$merge"]
	if m != nil {
		return processMapMerge(obj, mergeFrom, mergeFromDocs, m, depth)
	}

	m = obj["$replace"]
	if m != nil {
		return processMapReplace(mergeFrom, mergeFromDocs, m, depth)
	}

	encode := getMapStringValue(obj, "$encode")
	if encode != "" {
		delete(obj, "$encode")
	}

	keys := polyfill.MapsKeys(obj)
	polyfill.SlicesSort(keys)

	for _, k := range keys {
		v := obj[k]

		v2, err := process(v, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			delete(obj, k)
			continue
		}

		obj[k] = v2
	}

	if encode != "" {
		return processMapEncode(encode, obj)
	}

	return obj, nil
}

func processMapMerge(obj map[string]any, mergeFrom any, mergeFromDocs []any, m any, depth int) (any, error) {
	delete(obj, "$merge")

	in, err := get(mergeFrom, mergeFromDocs, m)
	if err != nil {
		return nil, err
	}

	next, err := mergeMap(obj, in)
	if err != nil {
		return nil, err
	}

	return process(next, mergeFrom, mergeFromDocs, depth)
}

func processMapReplace(mergeFrom any, mergeFromDocs []any, m any, depth int) (any, error) {
	next, err := get(mergeFrom, mergeFromDocs, m)
	if err != nil {
		return nil, err
	}

	return process(next, mergeFrom, mergeFromDocs, depth)
}

func processMapEncode(encode string, obj any) (string, error) {
	f, err := GetFormat(encode)
	if err != nil {
		return "", err
	}

	enc, err := f.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(enc), nil
}

func processList(obj []any, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	m, obj, err := popListMapValue(obj, "$merge")
	if err != nil {
		return nil, err
	}

	if m != nil {
		return processListMerge(obj, mergeFrom, mergeFromDocs, m, depth)
	}

	m, obj, err = popListMapValue(obj, "$replace")
	if err != nil {
		return nil, err
	}

	if m != nil {
		return processListReplace(mergeFrom, mergeFromDocs, m, depth)
	}

	encode, obj, err := popListMapStringValue(obj, "$encode")
	if err != nil {
		return nil, err
	}

	obj, err = filterList(obj, func(v any) ([]any, error) {
		v2, err := process(v, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			return nil, nil
		}

		return []any{v2}, nil
	})
	if err != nil {
		return nil, err
	}

	if encode != "" {
		return processListEncode(encode, obj)
	}

	return obj, nil
}

func processListMerge(obj []any, mergeFrom any, mergeFromDocs []any, m any, depth int) (any, error) {
	in, err := get(mergeFrom, mergeFromDocs, m)
	if err != nil {
		return nil, err
	}

	next, err := mergeList(obj, in)
	if err != nil {
		return nil, err
	}

	return process(next, mergeFrom, mergeFromDocs, depth)
}

func processListReplace(mergeFrom any, mergeFromDocs []any, m any, depth int) (any, error) {
	next, err := get(mergeFrom, mergeFromDocs, m)
	if err != nil {
		return nil, err
	}

	return process(next, mergeFrom, mergeFromDocs, depth)
}

func processListEncode(encode string, obj any) (any, error) {
	f, err := GetFormat(encode)
	if err != nil {
		return nil, err
	}

	enc, err := f.Marshal(obj)
	if err != nil {
		return nil, err
	}

	return string(enc), nil
}

func processString(obj string, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	if strings.HasPrefix(obj, "$merge:") {
		return processStringMerge(obj, mergeFrom, mergeFromDocs, depth)
	}

	if strings.HasPrefix(obj, "$replace:") {
		return processStringReplace(obj, mergeFrom, mergeFromDocs, depth)
	}

	return obj, nil
}

func processStringMerge(obj string, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	path := strings.TrimPrefix(obj, "$merge:")

	in, err := get(mergeFrom, mergeFromDocs, path)
	if err != nil {
		return nil, err
	}

	return process(in, mergeFrom, mergeFromDocs, depth)
}

func processStringReplace(obj string, mergeFrom any, mergeFromDocs []any, depth int) (any, error) {
	path := strings.TrimPrefix(obj, "$replace:")

	in, err := get(mergeFrom, mergeFromDocs, path)
	if err != nil {
		return nil, err
	}

	return process(in, mergeFrom, mergeFromDocs, depth)
}
