package bkl

import (
	"fmt"
	"unicode"

	"golang.org/x/exp/utf8string"
)

func validate(obj any) error {
	switch obj2 := obj.(type) {
	case map[string]any:
		return validateMap(obj2)

	case []any:
		return validateList(obj2)

	case string:
		return validateString(obj2)

	default:
		return nil
	}
}

func validateMap(obj map[string]any) error {
	for k, v := range obj {
		err := validate(k)
		if err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}

		err = validate(v)
		if err != nil {
			return fmt.Errorf("%s: %w", k, err)
		}
	}

	return nil
}

func validateList(obj []any) error {
	for _, v := range obj {
		err := validate(v)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateString(obj string) error {
	if obj == "$required" {
		return ErrRequiredField
	}

	us := utf8string.NewString(obj)
	if us.RuneCount() >= 2 && us.At(0) == '$' && unicode.IsLower(us.At(1)) {
		return fmt.Errorf("%s: %w", obj, ErrInvalidDirective)
	}

	return nil
}
