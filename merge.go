package bkl

import (
	"fmt"

	"golang.org/x/exp/slices"
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
		patch := getStringValue(src2, "$patch")
		if patch != "" {
			switch patch {
			case "replace":
				delete(src2, "$patch")
				return src2, nil

			default:
				return nil, fmt.Errorf("%s: %w", patch, ErrInvalidPatchValue)
			}
		}

		for k, v := range src2 {
			if v == nil {
				delete(dst, k)
				continue
			}

			existing, found := dst[k]
			if found {
				n, err := merge(existing, v)
				if err != nil {
					return nil, fmt.Errorf("%s %w", k, err)
				}

				dst[k] = n
			} else {
				dst[k] = v
			}
		}

		return dst, nil

	case nil:
		return dst, nil

	default:
		return nil, fmt.Errorf("merge map[string]any with %T: %w", src, ErrInvalidType)
	}
}

func mergeList(dst []any, src any) (any, error) {
	switch src2 := src.(type) {
	case []any:
		dst = slicesDeleteFunc(
			dst,
			func(v any) bool {
				return toString(v) == "$required"
			},
		)

		for i, val := range src2 {
			switch val2 := val.(type) { //nolint:gocritic
			case map[string]any:
				patch := getStringValue(val2, "$patch")
				if patch != "" {
					switch patch {
					case "delete":
						delete(val2, "$patch")

						dst = slicesDeleteFunc(dst, func(elem any) bool {
							return match(elem, val2)
						})

						continue

					case "replace":
						return slices.Delete(src2, i, i+1), nil

					default:
						return nil, fmt.Errorf("%s: %w", patch, ErrInvalidPatchValue)
					}
				}
			}

			dst = append(dst, val)
		}

		return dst, nil

	case nil:
		return dst, nil

	default:
		return nil, fmt.Errorf("merge []any with %T: %w", src, ErrInvalidType)
	}
}
