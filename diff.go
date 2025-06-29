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
	"github.com/gopatchy/bkl/internal/utils"
)

// Diff loads two files and returns the diff between them.
// It expects each file to contain exactly one document.
// The files are loaded directly without processing, matching bkld behavior.
// If format is nil, it infers the format from the paths parameter.
func Diff(fx fs.FS, srcPath, dstPath string, rootPath string, workingDir string, format *string, paths ...*string) ([]byte, error) {
	preparedPaths, err := utils.PreparePathsForParser([]string{srcPath, dstPath}, rootPath, workingDir)
	if err != nil {
		return nil, err
	}
	srcPath = preparedPaths[0]
	dstPath = preparedPaths[1]

	var srcDocs []*document.Document

	realSrcPath, _, err := fileMatch(fx, srcPath)
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

	if len(srcDocs) != 1 {
		return nil, fmt.Errorf("diff operates on exactly 1 source document per file, got %d", len(srcDocs))
	}

	var dstDocs []*document.Document

	realDstPath, _, err := fileMatch(fx, dstPath)
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

	if len(dstDocs) != 1 {
		return nil, fmt.Errorf("diff operates on exactly 1 destination document per file, got %d", len(dstDocs))
	}

	result, err := diff(dstDocs[0].Data, srcDocs[0].Data)
	if err != nil {
		return nil, err
	}

	var finalResult any
	switch result2 := result.(type) {
	case map[string]any:
		result2["$match"] = map[string]any{}
		finalResult = result2

	case []any:
		result2 = append([]any{
			map[string]any{
				"$match": map[string]any{},
			},
		}, result2...)
		finalResult = result2

	case nil:

		finalResult = map[string]any{"$match": map[string]any{}}

	default:
		finalResult = result
	}

	ft, err := determineFormat(format, paths...)
	if err != nil {
		return nil, err
	}
	return ft.MarshalStream([]any{finalResult})
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
