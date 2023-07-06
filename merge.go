package bkl

import (
	"fmt"
	"slices"
	"strings"
)

func merge(dst any, src any) (any, error) {
	switch dst2 := canonicalizeType(dst).(type) {
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

func mergeMap(dst map[string]any, src any) (any, error) {
	switch src2 := canonicalizeType(src).(type) {
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
	switch src2 := canonicalizeType(src).(type) {
	case []any:
		for i, val := range src2 {
			switch val2 := canonicalizeType(val).(type) { //nolint:gocritic
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

func canonicalizeType(in any) any {
	switch inType := in.(type) {
	case []map[string]any:
		ret := []any{}
		for _, val := range inType {
			ret = append(ret, val)
		}

		return ret

	default:
		return inType
	}
}

func match(obj any, pat any) bool {
	switch patType := canonicalizeType(pat).(type) {
	case map[string]any:
		objMap, ok := obj.(map[string]any)
		if !ok {
			return false
		}

		result := true

		for patKey, patVal := range patType {
			result = result && match(objMap[patKey], patVal)
		}

		return result

	case []any:
		objList, ok := obj.([]any)
		if !ok {
			return false
		}

		result := true

		for _, patVal := range patType {
			found := false

			for _, objVal := range objList {
				if match(objVal, patVal) {
					found = true
					break
				}
			}

			result = result && found
		}

		return result

	default:
		return obj == pat
	}
}

func get(obj any, path string) any {
	parts := strings.Split(path, ".")
	return getRecursive(obj, parts)
}

func getRecursive(obj any, parts []string) any {
	if len(parts) == 0 {
		return obj
	}

	switch objType := obj.(type) {
	case map[string]any:
		return getRecursive(objType[parts[0]], parts[1:])

	default:
		return nil
	}
}
