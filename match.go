package bkl

import (
	"github.com/gopatchy/bkl/internal/document"
	"github.com/gopatchy/bkl/internal/utils"
)

func matchDoc(doc *document.Document, pat any) bool {
	return match(doc.Data, pat)
}

func match(obj any, pat any) bool {
	switch pat2 := pat.(type) {
	case map[string]any:
		return matchMap(obj, pat2)

	case []any:
		return matchList(obj, pat2)

	default:
		return obj == pat
	}
}

func matchMap(obj any, pat map[string]any) bool {
	invert, pat := utils.PopMapBoolValue(pat, "$invert", true)
	if invert {
		return !matchMap(obj, pat)
	}

	objMap, ok := obj.(map[string]any)
	if !ok {
		return false
	}

	if len(objMap) == 1 {
		for k := range objMap {
			if k == "$merge" || k == "$replace" || k == "$encode" {
				return false
			}
		}
	}

	for pk, pv := range pat {
		if !match(objMap[pk], pv) {
			return false
		}
	}

	return true
}

func matchList(obj any, pat []any) bool {
	objList, ok := obj.([]any)
	if !ok {
		return false
	}

	for _, pv := range pat {
		if !matchListSingle(objList, pv) {
			return false
		}
	}

	return true
}

func matchListSingle(obj []any, pat any) bool {
	for _, ov := range obj {
		if match(ov, pat) {
			return true
		}
	}

	return false
}
