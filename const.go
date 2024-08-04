package bkl

import (
	"fmt"
	"strings"
)

func constEval(obj any, mergeFrom *Document) (any, error) {
	switch obj2 := obj.(type) {
	case map[string]any:
		return constEvalMap(obj2, mergeFrom)

	case []any:
		return constEvalList(obj2, mergeFrom)

	case string:
		return constEvalString(obj2, mergeFrom)

	default:
		return obj2, nil
	}
}

func constEvalMap(obj map[string]any, mergeFrom *Document) (map[string]any, error) {
	return filterMap(obj, func(k string, v any) (map[string]any, error) {
		ka, err := constEvalString(k, mergeFrom)
		if err != nil {
			return nil, err
		}

		v, err = constEval(v, mergeFrom)
		if err != nil {
			return nil, err
		}

		return map[string]any{fmt.Sprintf("%v", ka): v}, nil
	})
}

func constEvalList(obj []any, mergeFrom *Document) ([]any, error) {
	return filterList(obj, func(v any) ([]any, error) {
		v, err := constEval(v, mergeFrom)
		if err != nil {
			return nil, err
		}

		return []any{v}, nil
	})
}

func constEvalString(obj string, mergeFrom *Document) (any, error) {
	if strings.HasPrefix(obj, `$"`) && strings.HasSuffix(obj, `"`) {
		return constInterp(obj, mergeFrom)
	}

	ret, err := getVar(mergeFrom, obj)
	if err == nil {
		return ret, nil
	}

	return obj, nil
}

func constInterp(obj string, mergeFrom *Document) (any, error) {
	obj = interpRE.ReplaceAllStringFunc(obj, func(m string) string {
		m2 := strings.TrimSuffix(strings.TrimPrefix(m, `{`), `}`)

		v, err := getVar(mergeFrom, m2)
		if err != nil {
			return m
		}

		return fmt.Sprintf("%v", v)
	})

	return obj, nil
}
