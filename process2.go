package bkl

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/gopatchy/bkl/polyfill"
)

func process2(obj any, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	depth++

	if depth > 1000 {
		return nil, fmt.Errorf("%#v: %w", obj, ErrCircularRef)
	}

	switch obj2 := obj.(type) {
	case map[string]any:
		return process2Map(obj2, mergeFrom, mergeFromDocs, depth)

	case []any:
		return process2List(obj2, mergeFrom, mergeFromDocs, depth)

	case string:
		return process2String(obj2, mergeFrom, mergeFromDocs, depth)

	default:
		return obj, nil
	}
}

func process2Map(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	if found, v, obj := popMapValue(obj, "$encode"); found {
		return process2Encode(obj, mergeFrom, mergeFromDocs, v, depth)
	}

	if found, v, obj := popMapValue(obj, "$decode"); found {
		return process2Decode(obj, mergeFrom, mergeFromDocs, v, depth)
	}

	if found, v, obj := popMapValue(obj, "$value"); found {
		if len(obj) != 0 {
			return nil, fmt.Errorf("$value: %#v (%w)", obj, ErrExtraKeys)
		}

		return process2MapValue(obj, mergeFrom, mergeFromDocs, v, depth)
	}

	return filterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, err := process2(v, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			return map[string]any{}, nil
		}

		k2, err := process2(k, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		return map[string]any{k2.(string): v2}, nil
	})
}

func process2MapValue(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, v any, depth int) (any, error) {
	return process2(v, mergeFrom, mergeFromDocs, depth)
}

func process2Encode(obj any, mergeFrom *Document, mergeFromDocs []*Document, v any, depth int) (any, error) {
	obj2, err := process2(obj, mergeFrom, mergeFromDocs, depth)
	if err != nil {
		return nil, err
	}

	err = validate(obj2)
	if err != nil {
		return nil, err
	}

	return process2EncodeAny(obj2, mergeFrom, mergeFromDocs, v, depth)
}

func process2EncodeAny(obj any, mergeFrom *Document, mergeFromDocs []*Document, v any, depth int) (any, error) {
	switch v2 := v.(type) {
	case string:
		return process2EncodeString(obj, mergeFrom, mergeFromDocs, v2, depth)

	case []any:
		for _, v3 := range v2 {
			var err error

			obj, err = process2EncodeAny(obj, mergeFrom, mergeFromDocs, v3, depth)
			if err != nil {
				return nil, err
			}
		}

		return obj, nil

	default:
		return nil, fmt.Errorf("$encode: %T: %w", v, ErrInvalidType)
	}
}

func process2EncodeString(obj any, mergeFrom *Document, mergeFromDocs []*Document, v string, depth int) (any, error) {
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
		return process2EncodeAny(obj, mergeFrom, mergeFromDocs, []any{"tolist:=", "prefix:--"}, depth+1)

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
			return process2ToListList(obj2, delim)
		}

		return process2ToListMap(obj, delim)

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

func process2Decode(obj any, mergeFrom *Document, mergeFromDocs []*Document, v any, depth int) (any, error) {
	switch v2 := v.(type) {
	case string:
		return process2DecodeString(obj, mergeFrom, mergeFromDocs, v2, depth)

	default:
		return nil, fmt.Errorf("$decode: %T: %w", v, ErrInvalidType)
	}
}

func process2DecodeString(obj any, mergeFrom *Document, mergeFromDocs []*Document, v string, depth int) (any, error) {
	switch obj2 := obj.(type) {
	case map[string]any:
		return process2DecodeStringMap(obj2, mergeFrom, mergeFromDocs, v, depth)

	default:
		return nil, fmt.Errorf("$decode: %T: %w", obj, ErrInvalidType)
	}
}

func process2DecodeStringMap(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, v string, depth int) (any, error) {
	found, val, obj := popMapValue(obj, "$value")
	if !found {
		return nil, fmt.Errorf("$decode: missing $value in %#v (#w)", obj, ErrInvalidType)
	}

	val2, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("$value: %T (%w)", val, ErrInvalidType)
	}

	if len(obj) != 0 {
		return nil, fmt.Errorf("$value: %#v (%w)", obj, ErrExtraKeys)
	}

	f, err := GetFormat(v)
	if err != nil {
		return nil, err
	}

	decs, err := f.UnmarshalStream([]byte(val2))
	if err != nil {
		return nil, err
	}

	if len(decs) != 1 {
		return nil, fmt.Errorf("%#v (%w)", val2, ErrUnmarshal)
	}

	return process2(decs[0], mergeFrom, mergeFromDocs, depth)
}

func process2ToListList(obj []any, delim string) ([]any, error) {
	ret := []any{}

	for _, iter := range obj {
		vals, err := process2ToListMap(iter, delim)
		if err != nil {
			return nil, err
		}

		ret = append(ret, vals...)
	}

	return ret, nil
}

func process2ToListMap(obj any, delim string) ([]any, error) {
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

func process2List(obj []any, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	m, obj, err := popListMapValue(obj, "$encode")
	if err != nil {
		return nil, err
	}

	if m != nil {
		return process2Encode(obj, mergeFrom, mergeFromDocs, m, depth)
	}

	return filterList(obj, func(v any) ([]any, error) {
		v2, err := process2(v, mergeFrom, mergeFromDocs, depth)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			return nil, nil
		}

		return []any{v2}, nil
	})
}

func process2String(obj string, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	if strings.HasPrefix(obj, `$"`) && strings.HasSuffix(obj, `"`) {
		return process2StringInterp(obj, mergeFrom, mergeFromDocs, depth)
	}

	if strings.HasPrefix(obj, "$env:") || obj == "$repeat" {
		return getVar(mergeFrom, obj)
	}

	return obj, nil
}

var interpRE = regexp.MustCompile(`{.*?}`)

func process2StringInterp(obj string, mergeFrom *Document, mergeFromDocs []*Document, depth int) (any, error) {
	obj = strings.TrimSuffix(strings.TrimPrefix(obj, `$"`), `"`)

	var err error

	obj = interpRE.ReplaceAllStringFunc(obj, func(m string) string {
		if err != nil {
			return "{ERROR}"
		}

		m = strings.TrimSuffix(strings.TrimPrefix(m, `{`), `}`)

		var v any

		v, err = getWithVar(mergeFrom, mergeFromDocs, m)
		if err != nil {
			return "{ERROR}"
		}

		if v2, ok := v.(string); ok {
			v, err = process2String(v2, mergeFrom, mergeFromDocs, depth+1)
			if err != nil {
				return "{ERROR}"
			}
		}

		return fmt.Sprintf("%v", v)
	})

	if err != nil {
		return nil, err
	}

	return obj, nil
}
