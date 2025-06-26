package bkl

import (
	"fmt"
	"io/fs"
	"maps"
	"reflect"
	"slices"
)

// DiffFiles loads two files and returns the diff between them.
// It expects each file to contain exactly one document.
// The files are loaded with MergeFileLayers but not processed, matching bkld behavior.
func (b *BKL) DiffFiles(fsys fs.FS, srcPath, dstPath string) (any, error) {
	// Load source file
	p1, err := New()
	if err != nil {
		return nil, err
	}

	realSrcPath, _, err := p1.FileMatch(fsys, srcPath)
	if err != nil {
		return nil, fmt.Errorf("source file %s: %w", srcPath, err)
	}

	if err := p1.MergeFileLayers(fsys, realSrcPath); err != nil {
		return nil, fmt.Errorf("merging source %s: %w", srcPath, err)
	}

	srcDocs := p1.docs
	if len(srcDocs) != 1 {
		return nil, fmt.Errorf("diff operates on exactly 1 source document per file, got %d", len(srcDocs))
	}

	// Load destination file
	p2, err := New()
	if err != nil {
		return nil, err
	}

	realDstPath, _, err := p2.FileMatch(fsys, dstPath)
	if err != nil {
		return nil, fmt.Errorf("destination file %s: %w", dstPath, err)
	}

	if err := p2.MergeFileLayers(fsys, realDstPath); err != nil {
		return nil, fmt.Errorf("merging destination %s: %w", dstPath, err)
	}

	dstDocs := p2.docs
	if len(dstDocs) != 1 {
		return nil, fmt.Errorf("diff operates on exactly 1 destination document per file, got %d", len(dstDocs))
	}

	// Perform diff
	return b.diff(srcDocs[0].Data, dstDocs[0].Data, nil)
}

// diff generates the minimal intermediate layer needed to transform src into dst.
// It returns a document that, when merged with src, produces dst.
func (b *BKL) diff(src, dst any, env map[string]string) (any, error) {
	result, err := diff(dst, src)
	if err != nil {
		return nil, err
	}

	// Add $match directive at the appropriate level
	switch result2 := result.(type) {
	case map[string]any:
		result2["$match"] = map[string]any{}
		return result2, nil

	case []any:
		result2 = append([]any{
			map[string]any{
				"$match": map[string]any{},
			},
		}, result2...)
		return result2, nil

	case nil:
		// No differences - return just the match directive
		return map[string]any{"$match": map[string]any{}}, nil

	default:
		return result, nil
	}
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
		// Different types
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
