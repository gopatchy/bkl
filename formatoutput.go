package bkl

import (
	"github.com/gopatchy/bkl/internal/format"
	"github.com/gopatchy/bkl/internal/utils"
)

// FormatOutput marshals the given data to the specified format.
// If format is nil or points to an empty string, it looks at the provided paths
// and uses the file extension of the first non-nil path as the format.
// Returns the marshaled bytes or an error if the format is unknown or marshaling fails.
func FormatOutput(data any, format *string, paths ...*string) ([]byte, error) {
	ft, err := determineFormat(format, paths...)
	if err != nil {
		return nil, err
	}

	return ft.MarshalStream([]any{data})
}

// determineFormat determines the format to use based on the provided format pointer and paths.
// If format is nil or points to an empty string, it infers from the paths.
// Returns an error if no format can be determined.
func determineFormat(formatName *string, paths ...*string) (*format.Format, error) {
	if formatName != nil && *formatName != "" {
		return format.Get(*formatName)
	}

	// Try to infer from paths
	for _, path := range paths {
		if path != nil && *path != "" {
			if name := utils.Ext(*path); name != "" {
				return format.Get(name)
			}
		}
	}

	return format.Get("")
}
