package bkl

import "fmt"

var (
	// Base error; every error in bkl inherits from this
	Err = fmt.Errorf("bkl error")

	// Format and system errors
	ErrOutputFile      = fmt.Errorf("error opening output file (%w)", Err)
	ErrInvalidIndex    = fmt.Errorf("invalid index (%w)", Err)
	ErrInvalidFilename = fmt.Errorf("invalid filename (%w)", Err)
	ErrInvalidType     = fmt.Errorf("invalid type (%w)", Err)
	ErrMarshal         = fmt.Errorf("encoding error (%w)", Err)
	ErrMissingFile     = fmt.Errorf("missing file (%w)", Err)
	ErrNoMatchFound    = fmt.Errorf("no document/entry matched $match (%w)", Err)
	ErrRequiredField   = fmt.Errorf("required field not set (%w)", Err)
	ErrUnknownFormat   = fmt.Errorf("unknown format (%w)", Err)
	ErrUnmarshal       = fmt.Errorf("decoding error (%w)", Err)
	ErrUselessOverride = fmt.Errorf("useless override (%w)", Err)

	// Base language directive error
	ErrInvalidDirective = fmt.Errorf("invalid directive (%w)", Err)

	// Specific language directive errors
	ErrMergeRefNotFound   = fmt.Errorf("$merge reference not found (%w)", ErrInvalidDirective)
	ErrReplaceRefNotFound = fmt.Errorf("$replace reference not found (%w)", ErrInvalidDirective)
)
