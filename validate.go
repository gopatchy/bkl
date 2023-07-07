package bkl

import (
	"fmt"
	"strings"
)

func validate(obj any) error {
	switch obj2 := canonicalizeType(obj).(type) {
	case map[string]any:
		for k, v := range obj2 {
			err := validate(k)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}

			err = validate(v)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}

	case []any:
		for _, v := range obj2 {
			err := validate(v)
			if err != nil {
				return err
			}
		}

	case string:
		if obj2 == "$required" {
			return ErrRequiredField
		}

		if strings.HasPrefix(obj2, "$") {
			return fmt.Errorf("%s: %w", obj2, ErrInvalidDirective)
		}
	}

	return nil
}
