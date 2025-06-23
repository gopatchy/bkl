package bkl

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

func process2(obj any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, depth int) (any, error) {
	depth++

	if depth > 1000 {
		return nil, fmt.Errorf("%#v: %w", obj, ErrCircularRef)
	}

	switch obj2 := obj.(type) {
	case map[string]any:
		return process2Map(obj2, mergeFrom, mergeFromDocs, ec, depth)

	case []any:
		return process2List(obj2, mergeFrom, mergeFromDocs, ec, depth)

	case string:
		return process2String(obj2, mergeFrom, mergeFromDocs, ec, depth)

	default:
		return obj, nil
	}
}

func process2Map(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, depth int) (any, error) {
	obj, err := filterMap(obj, func(k string, v any) (map[string]any, error) {
		switch v2 := v.(type) {
		case map[string]any:
			if found, r, v3 := popMapValue(v2, "$repeat"); found {
				return process2RepeatObjMap(v3, mergeFrom, mergeFromDocs, ec, k, r, depth)
			}
		}

		return map[string]any{k: v}, nil
	})
	if err != nil {
		return nil, err
	}

	if found, v, obj := popMapValue(obj, "$encode"); found {
		return process2Encode(obj, mergeFrom, mergeFromDocs, ec, v, depth)
	}

	if found, v, obj := popMapValue(obj, "$decode"); found {
		return process2Decode(obj, mergeFrom, mergeFromDocs, ec, v, depth)
	}

	if found, v, obj := popMapValue(obj, "$value"); found {
		if len(obj) != 0 {
			return nil, fmt.Errorf("$value: %#v (%w)", obj, ErrExtraKeys)
		}

		return process2MapValue(obj, mergeFrom, mergeFromDocs, ec, v, depth)
	}

	return filterMap(obj, func(k string, v any) (map[string]any, error) {
		v2, err := process2(v, mergeFrom, mergeFromDocs, ec, depth)
		if err != nil {
			return nil, err
		}

		k2, err := process2(k, mergeFrom, mergeFromDocs, ec, depth)
		if err != nil {
			return nil, err
		}

		return map[string]any{k2.(string): v2}, nil
	})
}

func process2MapValue(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, v any, depth int) (any, error) {
	return process2(v, mergeFrom, mergeFromDocs, ec, depth)
}

func process2Encode(obj any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, v any, depth int) (any, error) {
	obj2, err := process2(obj, mergeFrom, mergeFromDocs, ec, depth)
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
			switch iter2 := iter.(type) {
			case []any:
				ret = append(ret, iter2...)

			default:
				ret = append(ret, iter)
			}
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

	case "sha256":
		if len(parts) != 1 {
			return nil, fmt.Errorf("$encode: %s: %w", v, ErrInvalidArguments)
		}

		sh := sha256.New()
		sh.Write([]byte(fmt.Sprintf("%v", obj)))
		return hex.EncodeToString(sh.Sum(nil)), nil

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

	case "values":
		if len(parts) != 1 {
			return nil, fmt.Errorf("$encode: %s: %w", v, ErrInvalidArguments)
		}

		obj2, ok := obj.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("$encode: %s: %w (%T)", v, ErrInvalidType, obj)
		}

		return process2ValuesMap(obj2)

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

func process2Decode(obj any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, v any, depth int) (any, error) {
	switch v2 := v.(type) {
	case string:
		return process2DecodeString(obj, mergeFrom, mergeFromDocs, ec, v2, depth)

	default:
		return nil, fmt.Errorf("$decode: %T: %w", v, ErrInvalidType)
	}
}

func process2DecodeString(obj any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, v string, depth int) (any, error) {
	switch obj2 := obj.(type) {
	case map[string]any:
		return process2DecodeStringMap(obj2, mergeFrom, mergeFromDocs, ec, v, depth)

	default:
		return nil, fmt.Errorf("$decode: %T: %w", obj, ErrInvalidType)
	}
}

func process2DecodeStringMap(obj map[string]any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, v string, depth int) (any, error) {
	found, val, obj := popMapValue(obj, "$value")
	if !found {
		return nil, fmt.Errorf("$decode: missing $value in %#v (%w)", obj, ErrInvalidType)
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

	// First normalize the decoded value
	normalized, err := normalize(decs[0])
	if err != nil {
		return nil, err
	}

	return process2(normalized, mergeFrom, mergeFromDocs, ec, depth)
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

	for k, v := range sortedMap(obj2) {
		switch v2 := v.(type) {
		case []any:
			for _, v3 := range v2 {
				ret = append(ret, process2ToListValue(k, delim, v3))
			}

		default:
			ret = append(ret, process2ToListValue(k, delim, v))
		}
	}

	return ret, nil
}

func process2ToListValue(k, delim string, v any) string {
	if v == "" {
		return k
	} else {
		return fmt.Sprintf("%s%s%v", k, delim, v)
	}
}

func process2List(obj []any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, depth int) (any, error) {
	m, obj, err := popListMapValue(obj, "$encode")
	if err != nil {
		return nil, err
	}

	if m != nil {
		return process2Encode(obj, mergeFrom, mergeFromDocs, ec, m, depth)
	}

	return filterList(obj, func(v any) ([]any, error) {
		switch v2 := v.(type) {
		case map[string]any:
			if found, r, v3 := popMapValue(v2, "$repeat"); found {
				return process2RepeatObjList(v3, mergeFrom, mergeFromDocs, ec, r, depth)
			}
		}

		v2, err := process2(v, mergeFrom, mergeFromDocs, ec, depth)
		if err != nil {
			return nil, err
		}

		return []any{v2}, nil
	})
}

func process2String(obj string, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, depth int) (any, error) {
	if strings.HasPrefix(obj, `$"`) && strings.HasSuffix(obj, `"`) {
		return process2StringInterp(obj, mergeFrom, mergeFromDocs, ec, depth)
	}

	if strings.HasPrefix(obj, "$env:") || obj == "$repeat" || strings.HasPrefix(obj, "$repeat:") {
		return ec.GetVar(obj)
	}

	return obj, nil
}

var interpRE = regexp.MustCompile(`{.*?}`)

func process2StringInterp(obj string, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, depth int) (any, error) {
	obj = strings.TrimSuffix(strings.TrimPrefix(obj, `$"`), `"`)

	var err error

	obj = interpRE.ReplaceAllStringFunc(obj, func(m string) string {
		if err != nil {
			return "{ERROR}"
		}

		m = strings.TrimSuffix(strings.TrimPrefix(m, `{`), `}`)

		var v any

		v, err = getWithVar(mergeFrom, mergeFromDocs, ec, m)
		if err != nil {
			return "{ERROR}"
		}

		if v2, ok := v.(string); ok {
			v, err = process2String(v2, mergeFrom, mergeFromDocs, ec, depth+1)
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

func process2ValuesMap(obj map[string]any) ([]any, error) {
	vals := []any{}

	for _, v := range sortedMap(obj) {
		vals = append(vals, v)
	}

	return vals, nil
}

func process2RepeatObjMap(v map[string]any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, k string, r any, depth int) (map[string]any, error) {
	ret := map[string]any{}

	contexts, err := repeatGenerateContexts(ec, r)
	if err != nil {
		return nil, err
	}

	for _, ctx := range contexts {
		v2, err := process2(v, mergeFrom, mergeFromDocs, ctx, depth)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			continue
		}

		k2, err := process2(k, mergeFrom, mergeFromDocs, ctx, depth)
		if err != nil {
			return nil, err
		}

		ret[k2.(string)] = v2
	}

	return ret, nil
}

func process2RepeatObjList(v map[string]any, mergeFrom *Document, mergeFromDocs []*Document, ec *evalContext, r any, depth int) ([]any, error) {
	ret := []any{}

	contexts, err := repeatGenerateContexts(ec, r)
	if err != nil {
		return nil, err
	}

	for _, ctx := range contexts {
		v2, err := process2(v, mergeFrom, mergeFromDocs, ctx, depth)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			continue
		}

		ret = append(ret, v2)
	}

	return ret, nil
}
