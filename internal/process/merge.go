package process

import (
	"fmt"

	"github.com/gopatchy/bkl/internal/document"

	"github.com/gopatchy/bkl/internal/utils"
	"github.com/gopatchy/bkl/pkg/errors"
)

func MergeDocs(doc, patch *document.Document) error {
	// If patch document is completely empty (nil), it's a no-op
	if patch.Data == nil {
		return nil
	}

	merged, err := merge(doc.Data, patch.Data)
	if err != nil {
		return err
	}

	doc.Data = merged
	patch.Parents = append(patch.Parents, doc)

	return nil
}

func merge(dst any, src any) (any, error) {
	switch dst2 := dst.(type) {
	case map[string]any:
		return mergeMap(dst2, src)

	case []any:
		return mergeList(dst2, src)

	case nil:
		return src, nil

	default:
		if src == dst {
			return nil, fmt.Errorf("%#v: %w", src, errors.ErrUselessOverride)
		}

		return src, nil
	}
}

func mergeMap(dst map[string]any, src any) (any, error) {
	switch src2 := src.(type) {
	case map[string]any:
		return mergeMapMap(dst, src2)

	default:
		return src, nil
	}
}

func mergeMapMap(dst map[string]any, src map[string]any) (map[string]any, error) {
	replace, found := utils.GetMapBoolValue(src, "$replace")
	if found && replace {
		delete(src, "$replace")
		return src, nil
	}

	for k, v := range src {
		existing, found := dst[k]

		if utils.ToString(v) == "$delete" {
			if !found {
				return nil, fmt.Errorf("%s=null: %w", k, errors.ErrUselessOverride)
			}

			delete(dst, k)
			continue
		}

		if found {
			v2, err := merge(existing, v)
			if err != nil {
				return nil, fmt.Errorf("%s %w", k, err)
			}

			dst[k] = v2
		} else {
			var err error

			dst[k], err = utils.DeepClone(v)
			if err != nil {
				return nil, err
			}
		}
	}

	return dst, nil
}

func mergeList(dst []any, src any) (any, error) {
	switch src2 := src.(type) {
	case []any:
		return mergeListList(dst, src2)

	default:
		return src, nil
	}
}

func mergeListList(dst []any, src []any) ([]any, error) {
	replace, src := utils.PopListString(src, "$replace")
	if replace {
		return src, nil
	}

	replace, src, err := utils.PopListMapBoolValue(src, "$replace", true)
	if err != nil {
		return nil, err
	}

	if replace {
		return src, nil
	}

	_, dst = utils.PopListString(dst, "$required")

	for _, v := range src {
		vMap, ok := v.(map[string]any)
		if !ok {
			dst = append(dst, v)
			continue
		}

		found, del, vMap := utils.PopMapValue(vMap, "$delete")
		if found {
			if len(vMap) > 0 {
				return nil, fmt.Errorf("%#v: %w", vMap, errors.ErrExtraKeys)
			}

			dst, err = mergeListDelete(dst, del)
			if err != nil {
				return nil, err
			}

			continue
		}

		found, m, vMap := utils.PopMapValue(vMap, "$match")
		if found {
			dst, err = mergeListMatch(dst, m, vMap)
			if err != nil {
				return nil, err
			}

			continue
		}

		found, matches, vMap := utils.PopMapValue(vMap, "$matches")
		if found {
			matchesList, ok := matches.([]any)
			if !ok {
				return nil, fmt.Errorf("$matches must be a list, got %T", matches)
			}

			for i, matchPattern := range matchesList {
				dst, err = mergeListMatch(dst, matchPattern, vMap)
				if err != nil {
					return nil, fmt.Errorf("$matches[%d]: %w", i, err)
				}
			}

			continue
		}

		dst = append(dst, v)
	}

	return dst, nil
}

func mergeListDelete(obj []any, del any) ([]any, error) {
	var err error

	deleted := false

	obj, err = utils.FilterList(obj, func(v any) ([]any, error) {
		if match(v, del) {
			deleted = true
			return nil, nil
		}

		return []any{v}, nil
	})
	if err != nil {
		return nil, err
	}

	if !deleted {
		return nil, fmt.Errorf("$delete: %#v: %w", del, errors.ErrUselessOverride)
	}

	return obj, nil
}

func mergeListMatch(obj []any, m any, v map[string]any) ([]any, error) {
	var val any = v

	found, v2, v := utils.PopMapValue(v, "$value")
	if found {
		if len(v) > 0 {
			return nil, fmt.Errorf("%#v: %w", v, errors.ErrExtraKeys)
		}

		val = v2
	}

	found = false

	obj, err := utils.FilterList(obj, func(v2 any) ([]any, error) {
		if match(v2, m) {
			found = true

			v2, err := merge(v2, val)
			if err != nil {
				return nil, err
			}

			return []any{v2}, nil
		}

		return []any{v2}, nil
	})
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, fmt.Errorf("%#v: %w", m, errors.ErrNoMatchFound)
	}

	return obj, nil
}
