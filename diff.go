package bkl

import (
	"fmt"
	"io/fs"
	"maps"
	"reflect"
	"slices"

	"github.com/gopatchy/bkl/internal/document"
	"github.com/gopatchy/bkl/internal/file"
	"github.com/gopatchy/bkl/internal/fsys"
	"github.com/gopatchy/bkl/internal/merge"
	"github.com/gopatchy/bkl/internal/path"
	"github.com/gopatchy/bkl/internal/utils"
)

func Diff(fx fs.FS, srcPath, dstPath string, rootPath string, workingDir string, selector string, format *string, paths ...*string) ([]byte, error) {
	preparedPaths, err := utils.PreparePathsForParser([]string{srcPath, dstPath}, rootPath, workingDir)
	if err != nil {
		return nil, err
	}
	srcPath = preparedPaths[0]
	dstPath = preparedPaths[1]

	var srcDocs []*document.Document

	realSrcPath, _, err := file.FileMatch(fx, srcPath)
	if err != nil {
		return nil, fmt.Errorf("source file %s: %w", srcPath, err)
	}

	fileObjs, err := file.LoadAndParents(fsys.New(fx), realSrcPath, nil)
	if err != nil {
		return nil, fmt.Errorf("loading source %s: %w", srcPath, err)
	}

	for _, f := range fileObjs {
		srcDocs, err = merge.FileObj(srcDocs, f)
		if err != nil {
			return nil, fmt.Errorf("merging source %s: %w", srcPath, err)
		}
	}

	var dstDocs []*document.Document

	realDstPath, _, err := file.FileMatch(fx, dstPath)
	if err != nil {
		return nil, fmt.Errorf("destination file %s: %w", dstPath, err)
	}

	fileSystem2 := fsys.New(fx)
	fileObjs2, err := file.LoadAndParents(fileSystem2, realDstPath, nil)
	if err != nil {
		return nil, fmt.Errorf("loading destination %s: %w", dstPath, err)
	}

	for _, f := range fileObjs2 {
		dstDocs, err = merge.FileObj(dstDocs, f)
		if err != nil {
			return nil, fmt.Errorf("merging destination %s: %w", dstPath, err)
		}
	}

	results := []any{}

	srcMap := make(map[string]*document.Document)
	var srcKeys []string
	for _, doc := range srcDocs {
		keyStr, err := evaluateSelector(doc, selector)
		if err != nil {
			return nil, fmt.Errorf("evaluating selector on source document: %w", err)
		}
		if _, exists := srcMap[keyStr]; exists {
			return nil, fmt.Errorf("selector %q matches multiple source documents", keyStr)
		}
		srcMap[keyStr] = doc
		srcKeys = append(srcKeys, keyStr)
	}

	dstMap := make(map[string]*document.Document)
	var dstKeys []string
	for _, doc := range dstDocs {
		keyStr, err := evaluateSelector(doc, selector)
		if err != nil {
			return nil, fmt.Errorf("evaluating selector on destination document: %w", err)
		}
		if _, exists := dstMap[keyStr]; exists {
			return nil, fmt.Errorf("selector %q matches multiple destination documents", keyStr)
		}
		dstMap[keyStr] = doc
		dstKeys = append(dstKeys, keyStr)
	}

	for _, keyStr := range dstKeys {
		dstDoc := dstMap[keyStr]
		srcDoc, found := srcMap[keyStr]
		if !found {
			switch d := dstDoc.Data.(type) {
			case map[string]any:
				d["$match"] = nil
				results = append(results, d)
			default:
				results = append(results, dstDoc.Data)
			}
		} else {
			result, err := diff(dstDoc.Data, srcDoc.Data)
			if err != nil {
				return nil, err
			}

			matchValue := map[string]any{}
			if selector != "" {
				parts := path.SplitPath(selector)
				val, _ := path.GetNoError(srcDoc.Data, parts)
				path.Set(matchValue, parts, val)
			}
			result = addMatchDirective(result, matchValue)
			results = append(results, result)
		}
	}

	for _, keyStr := range srcKeys {
		srcDoc := srcMap[keyStr]
		if _, found := dstMap[keyStr]; !found {
			matchValue := map[string]any{}
			if selector != "" {
				parts := path.SplitPath(selector)
				val, _ := path.GetNoError(srcDoc.Data, parts)
				path.Set(matchValue, parts, val)
			}

			result := map[string]any{
				"$match":  matchValue,
				"$output": false,
			}
			results = append(results, result)
		}
	}

	ft, err := determineFormat(format, paths...)
	if err != nil {
		return nil, err
	}
	return ft.MarshalStream(results)
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

func diffListList(dst, src []any) (any, error) {
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

func evaluateSelector(doc *document.Document, selector string) (string, error) {
	if selector == "" {
		return "", nil
	}
	parts := path.SplitPath(selector)
	val, err := path.GetNoError(doc.Data, parts)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(val), nil
}

func addMatchDirective(result any, matchValue map[string]any) any {
	switch r := result.(type) {
	case map[string]any:
		if _, hasMatch := r["$match"]; !hasMatch {
			r["$match"] = matchValue
		}
		return r
	case []any:
		return append([]any{
			map[string]any{
				"$match": matchValue,
			},
		}, r...)
	case nil:
		return map[string]any{"$match": matchValue}
	default:
		return result
	}
}
