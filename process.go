package bkl

import "fmt"

func process(root any) (any, error) {
	return processRecursive(root, root)
}

func processRecursive(root any, obj any) (any, error) {
	switch objType := obj.(type) {
	case map[string]any:
		if path, found := objType["$merge"]; found {
			delete(objType, "$merge")

			pathVal, ok := path.(string)
			if !ok {
				return nil, fmt.Errorf("%T: %w", path, ErrInvalidMergeType)
			}

			in := get(root, pathVal)
			if in == nil {
				return nil, fmt.Errorf("%s: (%w)", pathVal, ErrMergeRefNotFound)
			}

			next, err := merge(objType, in)
			if err != nil {
				return nil, err
			}

			return processRecursive(root, next)
		}

		if path, found := objType["$replace"]; found {
			delete(objType, "$replace")

			pathVal, ok := path.(string)
			if !ok {
				return nil, fmt.Errorf("%T: %w", path, ErrInvalidReplaceType)
			}

			next := get(root, pathVal)
			if next == nil {
				return nil, fmt.Errorf("%s: (%w)", pathVal, ErrReplaceRefNotFound)
			}

			return processRecursive(root, next)
		}

		for k, v := range objType {
			v2, err := processRecursive(root, v)
			if err != nil {
				return nil, err
			}

			objType[k] = v2
		}

		return objType, nil

	case []any:
		for i, v := range objType {
			v2, err := processRecursive(root, v)
			if err != nil {
				return nil, err
			}

			objType[i] = v2
		}

		return objType, nil

	default:
		return obj, nil
	}
}
