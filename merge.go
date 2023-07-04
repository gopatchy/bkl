package bkl

import (
	"fmt"
	"maps"
	"slices"
)

func Merge(dst any, src any) (any, error) {
	switch dst2 := CanonicalizeType(dst).(type) {
	case map[string]any:
		return MergeMap(dst2, src)

	case []any:
		return MergeList(dst2, src)

	case nil:
		return src, nil

	default:
		return src, nil
	}
}

func MergeMap(dst map[string]any, src any) (any, error) {
	switch src2 := CanonicalizeType(src).(type) {
	case map[string]any:
		if patch, found := src2["$patch"]; found {
			patchVal, ok := patch.(string)
			if !ok {
				return nil, fmt.Errorf("%T: %w", patch, ErrInvalidPatchType)
			}

			switch patchVal {
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
				n, err := Merge(existing, v)
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

func MergeList(dst []any, src any) (any, error) {
	switch src2 := CanonicalizeType(src).(type) {
	case []any:
		for i, val := range src2 {
			switch val2 := CanonicalizeType(val).(type) { //nolint:gocritic
			case map[string]any:
				if patch, found := val2["$patch"]; found {
					patchVal, ok := patch.(string)
					if !ok {
						return nil, fmt.Errorf("%T: %w", patch, ErrInvalidPatchType)
					}

					switch patchVal {
					case "delete":
						delete(val2, "$patch")

						dst = slices.DeleteFunc(dst, func(elem any) bool {
							switch elemType := elem.(type) {
							case map[string]any:
								return maps.Equal(elemType, val2)

							default:
								return false
							}
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

func CanonicalizeType(in any) any {
	switch t := in.(type) {
	case []map[string]any:
		ret := []any{}
		for _, v := range t {
			ret = append(ret, v)
		}

		return ret

	default:
		return in
	}
}
