package bkl

import (
	"fmt"
	"strings"

	"github.com/gopatchy/bkl/polyfill"
)

func Process(obj any, mergeFrom *Document, mergeFromDocs []*Document) (any, error) {
	return process(obj, mergeFrom, mergeFromDocs, 0)
}

// process() and descendants intentionally mutate obj to handle chained
// references
func process(obj any, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	depth++

	if depth > 1000 {
		return nil, fmt.Errorf("%#v: %w", obj, ErrCircularRef)
	}

	switch obj2 := obj.(type) {
	case map[string]any:
		return processMap(obj2, mergeFrom, mergeFromDocs, depth)

	case []any:
		return processList(obj2, mergeFrom, mergeFromDocs, depth)

	case string:
		return processString(obj2, mergeFrom, mergeFromDocs, depth)

	default:
		return obj, nil
	}
}

func processMap(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	if found, val, obj := popMapValue(obj, "$merge"); found {
		return processMapMerge(obj, mergeFrom, mergeFromDocs, val, depth)
	}

	if found, val, obj := popMapValue(obj, "$replace"); found {
		return processMapReplace(obj, mergeFrom, mergeFromDocs, val, depth)
	}

	if found, val, obj := popMapValue(obj, "$encode"); found {
		return processMapEncode(obj, mergeFrom, mergeFromDocs, val, depth)
	}

	if found, val, obj := popMapValue(obj, "$value"); found {
		return processMapValue(obj, mergeFrom, mergeFromDocs, val, depth)
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

	return obj, nil
}

func processMapMerge(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, val any, depth int) (any, error) {
	in, err := get(mergeFrom, mergeFromDocs, val)
	if err != nil {
		return nil, err
	}

	next, err := mergeMap(obj, in)
	if err != nil {
		return nil, err
	}

	return process(next, mergeFrom, mergeFromDocs, depth)
}

func processMapReplace(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, val any, depth int) (any, error) {
	next, err := get(mergeFrom, mergeFromDocs, val)
	if err != nil {
		return nil, err
	}

	return process(next, mergeFrom, mergeFromDocs, depth)
}

func processMapEncode(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, val any, depth int) (any, error) {
	switch val2 := val.(type) {
	case string:
		return processMapEncodeString(obj, mergeFrom, mergeFromDocs, val2, depth)

	default:
		return nil, fmt.Errorf("%T: %w", val, ErrInvalidType)
	}
}

func processMapEncodeString(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, val string, depth int) (any, error) {
	obj2, err := process(obj, mergeFrom, mergeFromDocs, depth)
	if err != nil {
		return nil, err
	}

	f, err := GetFormat(val)
	if err != nil {
		return "", err
	}

	enc, err := f.MarshalStream([]any{obj2})
	if err != nil {
		return "", err
	}

	return string(enc), nil
}

func processMapValue(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, val any, depth int) (any, error) {
	return process(val, mergeFrom, mergeFromDocs, depth)
}

func processList(obj []any, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
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

func processListMerge(obj []any, mergeFrom *Document, mergeFromDocs []*Document, m any, depth int) (any, error) {
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

func processListReplace(mergeFrom *Document, mergeFromDocs []*Document, m any, depth int) (any, error) {
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

	enc, err := f.MarshalStream([]any{obj})
	if err != nil {
		return nil, err
	}

	return string(enc), nil
}

func processString(obj string, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	if strings.HasPrefix(obj, "$merge:") {
		return processStringMerge(obj, mergeFrom, mergeFromDocs, depth)
	}

	if strings.HasPrefix(obj, "$replace:") {
		return processStringReplace(obj, mergeFrom, mergeFromDocs, depth)
	}

	return obj, nil
}

func processStringMerge(obj string, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	path := strings.TrimPrefix(obj, "$merge:")

	in, err := get(mergeFrom, mergeFromDocs, path)
	if err != nil {
		return nil, err
	}

	return process(in, mergeFrom, mergeFromDocs, depth)
}

func processStringReplace(obj string, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	path := strings.TrimPrefix(obj, "$replace:")

	in, err := get(mergeFrom, mergeFromDocs, path)
	if err != nil {
		return nil, err
	}

	return process(in, mergeFrom, mergeFromDocs, depth)
}
