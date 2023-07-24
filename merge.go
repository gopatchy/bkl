package bkl

import (
	"fmt"

	"github.com/gopatchy/bkl/polyfill"
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
	replace, src := popMapBoolValue(src, "$replace", true)
	if replace {
		return src, nil
	}

	dst = polyfill.MapsClone(dst)

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
	replace, src := popListString(src, "$replace")
	if replace {
		return src, nil
	}

	replace, src = popListMapBoolValue(src, "$replace", true)
	if replace {
		return src, nil
	}

	_, dst = popListString(dst, "$required")

	for _, v := range src {
		vMap, ok := v.(map[string]any)
		if ok {
			del, vMap := popMapValue(vMap, "$delete")
			if del != nil {
				deleted := false

				dst, _ = filterList(dst, func(v2 any) ([]any, error) {
					if match(v2, del) {
						deleted = true
						return nil, nil
					}

					return []any{v2}, nil
				})

				if !deleted {
					return nil, fmt.Errorf("%#v: %w", vMap, ErrUselessOverride)
				}

				continue
			}

			m, vMap := popMapValue(vMap, "$match")
			if m != nil {
				found := false

				dst, _ = filterList(dst, func(v2 any) ([]any, error) {
					if match(v2, m) {
						found = true

						v2, err := merge(v2, vMap)
						if err != nil {
							return nil, err
						}

						return []any{v2}, nil
					}

					return []any{v2}, nil
				})

				if !found {
					return nil, fmt.Errorf("%#v: %w", vMap, ErrNoMatchFound)
				}

				continue
			}
		}

		dst = append(dst, v)
	}

	return dst, nil
}
