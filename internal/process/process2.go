package process

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/gopatchy/bkl/internal/document"
	"github.com/gopatchy/bkl/internal/format"
	"github.com/gopatchy/bkl/internal/normalize"
	pathutil "github.com/gopatchy/bkl/internal/pathutil"
	"github.com/gopatchy/bkl/internal/utils"
	"github.com/gopatchy/bkl/pkg/errors"
)

func process2(obj any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, depth int) (any, error) {
	depth++

	if depth > 1000 {
		return nil, fmt.Errorf("%#v: %w", obj, errors.ErrCircularRef)
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

func process2Map(obj map[string]any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, depth int) (any, error) {
	obj, err := utils.FilterMap(obj, func(k string, v any) (map[string]any, error) {
		switch v2 := v.(type) {
		case map[string]any:
			if found, r, v3 := utils.PopMapValue(v2, "$repeat"); found {
				return process2RepeatObjMap(v3, mergeFrom, mergeFromDocs, ec, k, r, depth)
			}
		}

		return map[string]any{k: v}, nil
	})
	if err != nil {
		return nil, err
	}

	if found, v, obj := utils.PopMapValue(obj, "$encode"); found {
		return process2Encode(obj, mergeFrom, mergeFromDocs, ec, v, depth)
	}

	if found, v, obj := utils.PopMapValue(obj, "$decode"); found {
		return process2Decode(obj, mergeFrom, mergeFromDocs, ec, v, depth)
	}

	if found, v, obj := utils.PopMapValue(obj, "$value"); found {
		if len(obj) != 0 {
			return nil, fmt.Errorf("$value: %#v (%w)", obj, errors.ErrExtraKeys)
		}

		return process2MapValue(obj, mergeFrom, mergeFromDocs, ec, v, depth)
	}

	return utils.FilterMap(obj, func(k string, v any) (map[string]any, error) {
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

func process2MapValue(obj map[string]any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, v any, depth int) (any, error) {
	return process2(v, mergeFrom, mergeFromDocs, ec, depth)
}

func process2Encode(obj any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, v any, depth int) (any, error) {
	obj2, err := process2(obj, mergeFrom, mergeFromDocs, ec, depth)
	if err != nil {
		return nil, err
	}

	err = Validate(obj2)
	if err != nil {
		return nil, err
	}

	return process2EncodeAny(obj2, mergeFrom, mergeFromDocs, v, depth)
}

func process2EncodeAny(obj any, mergeFrom *document.Document, mergeFromDocs []*document.Document, v any, depth int) (any, error) {
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
		return nil, fmt.Errorf("$encode: %T: %w", v, errors.ErrInvalidType)
	}
}

func process2EncodeString(obj any, mergeFrom *document.Document, mergeFromDocs []*document.Document, v string, depth int) (any, error) {
	parts := strings.Split(v, ":")
	cmd := parts[0]

	switch cmd {
	case "base64":
		if len(parts) != 1 {
			return nil, fmt.Errorf("$encode: %s: %w", v, errors.ErrInvalidArguments)
		}

		obj2 := fmt.Sprintf("%v", obj)
		return base64.StdEncoding.EncodeToString([]byte(obj2)), nil

	case "flags":
		return process2EncodeAny(obj, mergeFrom, mergeFromDocs, []any{"tolist:=", "prefix:--"}, depth+1)

	case "flatten":
		if len(parts) != 1 {
			return nil, fmt.Errorf("$encode: %s: %w", v, errors.ErrInvalidArguments)
		}

		obj2, ok := obj.([]any)
		if !ok {
			return nil, fmt.Errorf("$encode: %s of non-list %T: %w", v, obj, errors.ErrInvalidType)
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
			return nil, fmt.Errorf("$encode: %s: %w", v, errors.ErrInvalidArguments)
		}

		strs, err := utils.ToStringListPermissive(obj)
		if err != nil {
			return nil, fmt.Errorf("$encode: %s: %w", v, err)
		}

		return strings.Join(strs, delim), nil

	case "prefix":
		if len(parts) != 2 {
			return nil, fmt.Errorf("$encode: %s: %w", v, errors.ErrInvalidArguments)
		}

		prefix := parts[1]

		strs, err := utils.ToStringListPermissive(obj)
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
			return nil, fmt.Errorf("$encode: %s: %w", v, errors.ErrInvalidArguments)
		}

		sh := sha256.New()
		if _, err := sh.Write([]byte(fmt.Sprintf("%v", obj))); err != nil {
			return nil, err
		}
		return hex.EncodeToString(sh.Sum(nil)), nil

	case "tolist":
		if len(parts) != 2 {
			return nil, fmt.Errorf("$encode: %s: %w", v, errors.ErrInvalidArguments)
		}

		delim := parts[1]

		obj2, ok := obj.([]any)
		if ok {
			return process2ToListList(obj2, delim)
		}

		return process2ToListMap(obj, delim)

	case "values":
		var nameKey, valueKey string

		switch len(parts) {
		case 1:
			// $encode: values
		case 2:
			// $encode: values:name
			nameKey = parts[1]
		case 3:
			// $encode: values:name:value
			nameKey = parts[1]
			valueKey = parts[2]
		default:
			return nil, fmt.Errorf("$encode: %s: %w", v, errors.ErrInvalidArguments)
		}

		obj2, ok := obj.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("$encode: %s: %w (%T)", v, errors.ErrInvalidType, obj)
		}

		return process2ValuesMap(obj2, nameKey, valueKey)

	default:
		if len(parts) != 1 {
			return nil, fmt.Errorf("$encode: %s: %w", v, errors.ErrInvalidArguments)
		}

		ft, err := format.Get(cmd)
		if err != nil {
			return nil, err
		}

		enc, err := ft.MarshalStream([]any{obj})
		if err != nil {
			return nil, err
		}

		return string(enc), nil
	}
}

func process2Decode(obj any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, v any, depth int) (any, error) {
	switch v2 := v.(type) {
	case string:
		return process2DecodeString(obj, mergeFrom, mergeFromDocs, ec, v2, depth)

	default:
		return nil, fmt.Errorf("$decode: %T: %w", v, errors.ErrInvalidType)
	}
}

func process2DecodeString(obj any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, v string, depth int) (any, error) {
	switch obj2 := obj.(type) {
	case map[string]any:
		return process2DecodeStringMap(obj2, mergeFrom, mergeFromDocs, ec, v, depth)

	default:
		return nil, fmt.Errorf("$decode: %T: %w", obj, errors.ErrInvalidType)
	}
}

func process2DecodeStringMap(obj map[string]any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, v string, depth int) (any, error) {
	found, val, obj := utils.PopMapValue(obj, "$value")
	if !found {
		return nil, fmt.Errorf("$decode: missing $value in %#v (%w)", obj, errors.ErrInvalidType)
	}

	val2, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("$value: %T (%w)", val, errors.ErrInvalidType)
	}

	if len(obj) != 0 {
		return nil, fmt.Errorf("$value: %#v (%w)", obj, errors.ErrExtraKeys)
	}

	ft, err := format.Get(v)
	if err != nil {
		return nil, err
	}

	decs, err := ft.UnmarshalStream([]byte(val2))
	if err != nil {
		return nil, err
	}

	if len(decs) != 1 {
		return nil, fmt.Errorf("%#v (%w)", val2, errors.ErrUnmarshal)
	}

	// First normalize the decoded value
	normalized, err := normalize.Document(decs[0])
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
		return nil, fmt.Errorf("$encode: tolist of non-map %#v: %w", obj, errors.ErrInvalidType)
	}

	ret := []any{}

	for k, v := range utils.SortedMap(obj2) {
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

func process2List(obj []any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, depth int) (any, error) {
	m, obj, err := utils.PopListMapValue(obj, "$encode")
	if err != nil {
		return nil, err
	}

	if m != nil {
		return process2Encode(obj, mergeFrom, mergeFromDocs, ec, m, depth)
	}

	return utils.FilterList(obj, func(v any) ([]any, error) {
		switch v2 := v.(type) {
		case map[string]any:
			if found, r, v3 := utils.PopMapValue(v2, "$repeat"); found {
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

func process2String(obj string, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, depth int) (any, error) {
	if strings.HasPrefix(obj, `$"`) && strings.HasSuffix(obj, `"`) {
		return process2StringInterp(obj, mergeFrom, mergeFromDocs, ec, depth)
	}

	if strings.HasPrefix(obj, "$env:") || obj == "$repeat" || strings.HasPrefix(obj, "$repeat:") {
		return ec.getVar(obj)
	}

	return obj, nil
}

var interpRE = regexp.MustCompile(`{.*?}`)

func process2StringInterp(obj string, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, depth int) (any, error) {
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

func process2ValuesMap(obj map[string]any, nameKey string, valueKey string) ([]any, error) {
	vals := []any{}

	switch {
	case nameKey == "" && valueKey == "":
		// $encode: values
		for _, v := range utils.SortedMap(obj) {
			vals = append(vals, v)
		}

	case nameKey != "" && valueKey == "":
		// $encode: values:name
		parts := pathutil.SplitPath(nameKey)
		for k, v := range utils.SortedMap(obj) {
			vMap, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("$encode: values:%s: value for key %q: %w (%T)", nameKey, k, errors.ErrInvalidType, v)
			}
			newMap := maps.Clone(vMap)
			pathutil.Set(newMap, parts, k)
			vals = append(vals, newMap)
		}

	case nameKey == "" && valueKey != "":
		// $encode: values::value
		valueParts := pathutil.SplitPath(valueKey)
		for _, v := range utils.SortedMap(obj) {
			newMap := map[string]any{}
			pathutil.Set(newMap, valueParts, v)
			vals = append(vals, newMap)
		}

	case nameKey != "" && valueKey != "":
		// $encode: values:name:value
		nameParts := pathutil.SplitPath(nameKey)
		valueParts := pathutil.SplitPath(valueKey)
		for k, v := range utils.SortedMap(obj) {
			newMap := map[string]any{}
			pathutil.Set(newMap, nameParts, k)
			pathutil.Set(newMap, valueParts, v)
			vals = append(vals, newMap)
		}

	default:
		return nil, fmt.Errorf("$encode: values with invalid combination of nameKey and valueKey: %w", errors.ErrInvalidArguments)
	}

	return vals, nil
}

func process2RepeatObjMap(v map[string]any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, k string, r any, depth int) (map[string]any, error) {
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

func process2RepeatObjList(v map[string]any, mergeFrom *document.Document, mergeFromDocs []*document.Document, ec *evalContext, r any, depth int) ([]any, error) {
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
