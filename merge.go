package bkl

import (
	"fmt"
)

func mergeDocs(doc, patch *Document) error {
	merged, err := merge(doc.Data, patch.Data)
	if err != nil {
		return err
	}

	doc.Data = merged
	patch.Parents = append(patch.Parents, doc)

	for k, v := range patch.Vars {
		doc.Vars[k] = v
	}

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
			return nil, fmt.Errorf("%#v: %w", src, ErrUselessOverride)
		}

		return src, nil
	}
}

func mergeMap(dst map[string]any, src any) (any, error) {
	switch src2 := src.(type) {
	case map[string]any:
		return mergeMapMap(dst, src2)

	case nil:
		return dst, nil

	default:
		if len(dst) == 0 {
			return src, nil
		}

		return nil, fmt.Errorf("merge map[string]any with %T: %w", src, ErrInvalidType)
	}
}

func mergeMapMap(dst map[string]any, src map[string]any) (map[string]any, error) {
	replace, found := getMapBoolValue(src, "$replace")
	if found && replace {
		delete(src, "$replace")
		return src, nil
	}

	for k, v := range src {
		existing, found := dst[k]

		if toString(v) == "$delete" {
			if !found {
				return nil, fmt.Errorf("%s=null: %w", k, ErrUselessOverride)
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
			dst[k] = v
		}
	}

	return dst, nil
}

func mergeList(dst []any, src any) (any, error) {
	switch src2 := src.(type) {
	case []any:
		return mergeListList(dst, src2)

	case nil:
		return dst, nil

	default:
		return nil, fmt.Errorf("merge []any with %T: %w", src, ErrInvalidType)
	}
}

func mergeListList(dst []any, src []any) ([]any, error) {
	replace, src := popListString(src, "$replace")
	if replace {
		return src, nil
	}

	replace, src, err := popListMapBoolValue(src, "$replace", true)
	if err != nil {
		return nil, err
	}

	if replace {
		return src, nil
	}

	_, dst = popListString(dst, "$required")

	for _, v := range src {
		vMap, ok := v.(map[string]any)
		if !ok {
			dst = append(dst, v)
			continue
		}

		found, del, vMap := popMapValue(vMap, "$delete")
		if found {
			if len(vMap) > 0 {
				return nil, fmt.Errorf("%#v: %w", vMap, ErrExtraKeys)
			}

			dst, err = mergeListDelete(dst, del)
			if err != nil {
				return nil, err
			}

			continue
		}

		found, m, vMap := popMapValue(vMap, "$match")
		if found {
			dst, err = mergeListMatch(dst, m, vMap)
			if err != nil {
				return nil, err
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

	obj, err = filterList(obj, func(v any) ([]any, error) {
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
		return nil, fmt.Errorf("$delete: %#v: %w", del, ErrUselessOverride)
	}

	return obj, nil
}

func mergeListMatch(obj []any, m any, v map[string]any) ([]any, error) {
	var val any = v

	found, v2, v := popMapValue(v, "$value")
	if found {
		if len(v) > 0 {
			return nil, fmt.Errorf("%#v: %w", v, ErrExtraKeys)
		}

		val = v2
	}

	found = false

	obj, err := filterList(obj, func(v2 any) ([]any, error) {
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
		return nil, fmt.Errorf("%#v: %w", m, ErrNoMatchFound)
	}

	return obj, nil
}
