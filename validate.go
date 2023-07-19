package bkl

import (
	"fmt"
	"strings"
)

func validate(obj any, required bool) error {
	switch obj2 := obj.(type) {
	case map[string]any:
		for k, v := range obj2 {
			err := validate(k, required)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}

			err = validate(v, required)
			if err != nil {
				return fmt.Errorf("%s: %w", k, err)
			}
		}

	case []any:
		for _, v := range obj2 {
			err := validate(v, required)
			if err != nil {
				return err
			}
		}

	case string:
		if obj2 == "$required" {
			if required {
				return ErrRequiredField
			}
		} else if strings.HasPrefix(obj2, "$") {
			return fmt.Errorf("%s: %w", obj2, ErrInvalidDirective)
		}
	}

	return nil
}
