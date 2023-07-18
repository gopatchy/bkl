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
		if src == dst {
			return nil, fmt.Errorf("%v: %w", src, ErrUselessOverride)
		}

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
		existing, found := dst[k]

		if v == nil {
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
	patch := listGetStringValue(src, "$patch")
	if patch == "replace" {
		_, src = listPopStringValue(src, "$patch")
		return src, nil
	}

	dst, _ = filterList(dst, func(v any) ([]any, error) {
		if toString(v) == "$required" {
			return nil, nil
		}

		return []any{v}, nil
	})

	for _, v := range src {
		vMap, ok := v.(map[string]any)
		if ok {
			patch, vMap := popStringValue(vMap, "$patch")
			switch patch {
			case "":

			case "delete":
				deleted := false

				dst, _ = filterList(dst, func(v2 any) ([]any, error) {
					if match(v2, vMap) {
						deleted = true
						return nil, nil
					}

					return []any{v2}, nil
				})

				if !deleted {
					return nil, fmt.Errorf("%#v: %w", vMap, ErrUselessOverride)
				}

				continue

			default:
				return nil, fmt.Errorf("%s: %w", patch, ErrInvalidPatchValue)
			}
		}

		dst = append(dst, v)
	}

	return dst, nil
}
