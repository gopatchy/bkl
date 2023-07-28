package bkl

import (
	"fmt"
	"unicode"

	"golang.org/x/exp/utf8string"
)

func validate(obj any) error {
	// TODO: Clean up
	switch obj2 := obj.(type) {
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

		us := utf8string.NewString(obj2)
		if us.RuneCount() >= 2 && us.At(0) == '$' && unicode.IsLower(us.At(1)) {
			return fmt.Errorf("%s: %w", obj2, ErrInvalidDirective)
		}
	}

	return nil
}
