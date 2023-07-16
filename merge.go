package bkl

import (
	"fmt"
)

func merge(dst any, src any) (any, error) {
	switch dst2 := dst.(type) {
	case map[string]any:
		return mergeMap(dst2, src)

	case []any:
		return mergeList(dst2, src)

	case nil:
		return src, nil

	default:
		return src, nil
	}
}

func mergeMap(dst map[string]any, src any) (map[string]any, error) {
	switch src2 := src.(type) {
	case map[string]any:
		return mergeMapMap(dst, src2)

	case nil:
		return dst, nil

	default:
		return nil, fmt.Errorf("merge map[string]any with %T: %w", src, ErrInvalidType)
	}
}

func mergeMapMap(dst map[string]any, src map[string]any) (map[string]any, error) {
	patch, src := popStringValue(src, "$patch")
	switch patch {
	case "":

	case "replace":
		return src, nil

	default:
		return nil, fmt.Errorf("%s: %w", patch, ErrInvalidPatchValue)
	}

	dst = mapsClone(dst)

	for k, v := range src {
		if v == nil {
			delete(dst, k)
			continue
		}

		existing, found := dst[k]
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
	patch := listGetStringValue(src, "$patch")
	if patch == "replace" {
		_, src = listPopStringValue(src, "$patch")
		return src, nil
	}

	dst = slicesClone(dst)

	dst = slicesDeleteFunc(
		dst,
		func(v any) bool {
			return toString(v) == "$required"
		},
	)

	for _, v := range src {
		vMap, ok := v.(map[string]any)
		if ok {
			patch, vMap := popStringValue(vMap, "$patch")
			switch patch {
			case "":

			case "delete":
				dst = slicesDeleteFunc(dst, func(elem any) bool {
					return match(elem, vMap)
				})

				continue

			default:
				return nil, fmt.Errorf("%s: %w", patch, ErrInvalidPatchValue)
			}
		}

		dst = append(dst, v)
	}

	return dst, nil
}
