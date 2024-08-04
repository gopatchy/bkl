package bkl

import (
	"encoding/base64"
	"fmt"
	"regexp"
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
	// Not copying obj before merge preserves the layering behavior that
	// tests/merge-race relies upon.
	if v, found := obj["$merge"]; found {
		delete(obj, "$merge")
		return processMapMerge(obj, mergeFrom, mergeFromDocs, v, depth)
	}

	if found, v, obj := popMapValue(obj, "$replace"); found {
		return processMapReplace(obj, mergeFrom, mergeFromDocs, v, depth)
	}

	if found, v, obj := popMapValue(obj, "$encode"); found {
		return processEncode(obj, mergeFrom, mergeFromDocs, v, depth)
	}

	if found, v, obj := popMapValue(obj, "$value"); found {
		return processMapValue(obj, mergeFrom, mergeFromDocs, v, depth)
	}

	keys := polyfill.MapsKeys(obj)
	polyfill.SlicesSort(keys)

	for _, k := range keys {
		v := obj[k]
		delete(obj, k)

		v2, err := process(v, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			delete(obj, k)
			continue
		}

		k2, err := process(k, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		obj[k2.(string)] = v2
	}

	return obj, nil
}

func processMapMerge(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, v any, depth int) (any, error) {
	in, err := get(mergeFrom, mergeFromDocs, v)
	if err != nil {
		return nil, err
	}

	next, err := mergeMap(obj, in)
	if err != nil {
		return nil, err
	}

	return process(next, mergeFrom, mergeFromDocs, depth)
}

func processMapReplace(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, v any, depth int) (any, error) {
	next, err := get(mergeFrom, mergeFromDocs, v)
	if err != nil {
		return nil, err
	}

	return process(next, mergeFrom, mergeFromDocs, depth)
}

func processMapValue(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, v any, depth int) (any, error) {
	return process(v, mergeFrom, mergeFromDocs, depth)
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
		return processListReplace(obj, mergeFrom, mergeFromDocs, m, depth)
	}

	m, obj, err = popListMapValue(obj, "$encode")
	if err != nil {
		return nil, err
	}

	if m != nil {
		return processEncode(obj, mergeFrom, mergeFromDocs, m, depth)
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

func processListReplace(obj []any, mergeFrom *Document, mergeFromDocs []*Document, m any, depth int) (any, error) {
	next, err := get(mergeFrom, mergeFromDocs, m)
	if err != nil {
		return nil, err
	}

	return process(next, mergeFrom, mergeFromDocs, depth)
}

func processString(obj string, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	if strings.HasPrefix(obj, "$merge:") {
		return processStringMerge(obj, mergeFrom, mergeFromDocs, depth)
	}

	if strings.HasPrefix(obj, "$replace:") {
		return processStringReplace(obj, mergeFrom, mergeFromDocs, depth)
	}

	if strings.HasPrefix(obj, `$"`) && strings.HasSuffix(obj, `"`) {
		return processStringInterp(obj, mergeFrom, mergeFromDocs, depth)
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

var interpRE = regexp.MustCompile(`{.*?}`)

func processStringInterp(obj string, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	obj = strings.TrimSuffix(strings.TrimPrefix(obj, `$"`), `"`)

	var err error

	obj = interpRE.ReplaceAllStringFunc(obj, func(m string) string {
		if err != nil {
			return "{ERROR}"
		}

		m = strings.TrimSuffix(strings.TrimPrefix(m, `{`), `}`)

		var v any

		v, err = get(mergeFrom, mergeFromDocs, m)
		if err != nil {
			return "{ERROR}"
		}

		return fmt.Sprintf("%v", v)
	})

	if err != nil {
		return nil, err
	}

	return obj, nil
}

func processEncode(obj any, mergeFrom *Document, mergeFromDocs []*Document, v any, depth int) (any, error) {
	obj2, err := process(obj, mergeFrom, mergeFromDocs, depth)
	if err != nil {
		return nil, err
	}

	err = validate(obj2)
	if err != nil {
		return nil, err
	}

	return processEncodeAny(obj2, mergeFrom, mergeFromDocs, v, depth)
}

func processEncodeAny(obj any, mergeFrom *Document, mergeFromDocs []*Document, v any, depth int) (any, error) {
	switch v2 := v.(type) {
	case string:
		return processEncodeString(obj, mergeFrom, mergeFromDocs, v2, depth)

	case []any:
		for _, v3 := range v2 {
			var err error

			obj, err = processEncodeAny(obj, mergeFrom, mergeFromDocs, v3, depth)
			if err != nil {
				return nil, err
			}
		}

		return obj, nil

	default:
		return nil, fmt.Errorf("$encode: %T: %w", v, ErrInvalidType)
	}
}

func processEncodeString(obj any, mergeFrom *Document, mergeFromDocs []*Document, v string, depth int) (any, error) {
	parts := strings.Split(v, ":")
	cmd := parts[0]

	switch cmd {
	case "base64":
		if len(parts) != 1 {
			return nil, fmt.Errorf("$encode: %s: %w", v, ErrInvalidArguments)
		}

		obj2 := fmt.Sprintf("%v", obj)
		return base64.StdEncoding.EncodeToString([]byte(obj2)), nil

	case "flags":
		return processEncodeAny(obj, mergeFrom, mergeFromDocs, []any{"tolist:=", "prefix:--"}, depth+1)

	case "flatten":
		if len(parts) != 1 {
			return nil, fmt.Errorf("$encode: %s: %w", v, ErrInvalidArguments)
		}

		obj2, ok := obj.([]any)
		if !ok {
			return nil, fmt.Errorf("$encode: %s of non-list %T: %w", v, obj, ErrInvalidType)
		}

		ret := []any{}

		for _, iter := range obj2 {
			iter2, ok := iter.([]any)
			if !ok {
				ret = append(ret, iter)
			}

			ret = append(ret, iter2...)
		}

		return ret, nil

	case "join":
		delim := ""

		if len(parts) == 2 {
			delim = parts[1]
		} else if len(parts) != 1 {
			return nil, fmt.Errorf("$encode: %s: %w", v, ErrInvalidArguments)
		}

		strs, err := toStringListPermissive(obj)
		if err != nil {
			return nil, fmt.Errorf("$encode: %s: %w", v, err)
		}

		return strings.Join(strs, delim), nil

	case "prefix":
		if len(parts) != 2 {
			return nil, fmt.Errorf("$encode: %s: %w", v, ErrInvalidArguments)
		}

		prefix := parts[1]

		strs, err := toStringListPermissive(obj)
		if err != nil {
			return nil, fmt.Errorf("$encode: %s: %w", v, err)
		}

		ret := []any{}

		for _, str := range strs {
			ret = append(ret, fmt.Sprintf("%s%s", prefix, str))
		}

		return ret, nil

	case "tolist":
		if len(parts) != 2 {
			return nil, fmt.Errorf("$encode: %s: %w", v, ErrInvalidArguments)
		}

		delim := parts[1]

		obj2, ok := obj.([]any)
		if ok {
			return processToListList(obj2, delim)
		}

		return processToListMap(obj, delim)

	default:
		if len(parts) != 1 {
			return nil, fmt.Errorf("$encode: %s: %w", v, ErrInvalidArguments)
		}

		f, err := GetFormat(cmd)
		if err != nil {
			return nil, err
		}

		enc, err := f.MarshalStream([]any{obj})
		if err != nil {
			return nil, err
		}

		return string(enc), nil
	}
}

func processToListList(obj []any, delim string) ([]any, error) {
	ret := []any{}

	for _, iter := range obj {
		vals, err := processToListMap(iter, delim)
		if err != nil {
			return nil, err
		}

		ret = append(ret, vals...)
	}

	return ret, nil
}

func processToListMap(obj any, delim string) ([]any, error) {
	obj2, ok := obj.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("$encode: tolist of non-map %#v: %w", obj, ErrInvalidType)
	}

	ret := []any{}

	keys := polyfill.MapsKeys(obj2)
	polyfill.SlicesSort(keys)

	for _, k := range keys {
		v := obj2[k]

		if v2, ok := v.(string); ok && v2 == "" {
			ret = append(ret, k)
		} else {
			ret = append(ret, fmt.Sprintf("%s%s%v", k, delim, v))
		}
	}

	return ret, nil
}
