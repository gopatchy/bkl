package bkl

import "fmt"

var (
	// Base error; every error in bkl inherits from this
	Err = fmt.Errorf("bkl error")

	// Format and system errors
	ErrDecode          = fmt.Errorf("decoding error (%w)", Err)
	ErrEncode          = fmt.Errorf("encoding error (%w)", Err)
	ErrOutputFile      = fmt.Errorf("error opening output file (%w)", Err)
	ErrInvalidIndex    = fmt.Errorf("invalid index (%w)", Err)
	ErrInvalidFilename = fmt.Errorf("invalid filename (%w)", Err)
	ErrInvalidType     = fmt.Errorf("invalid type (%w)", Err)
	ErrMissingFile     = fmt.Errorf("missing file (%w)", Err)
	ErrNoMatchFound    = fmt.Errorf("no document matched $match (%w)", Err)
	ErrRequiredField   = fmt.Errorf("required field not set (%w)", Err)
	ErrUnknownFormat   = fmt.Errorf("unknown format (%w)", Err)
	ErrUselessOverride = fmt.Errorf("useless override (%w)", Err)

	// Base language directive error
	ErrInvalidDirective = fmt.Errorf("invalid directive (%w)", Err)

	// Specific language directive errors
	ErrInvalidPatchValue  = fmt.Errorf("invalid $patch value (%w)", ErrInvalidDirective)
	ErrMergeRefNotFound   = fmt.Errorf("$merge reference not found (%w)", ErrInvalidDirective)
	ErrReplaceRefNotFound = fmt.Errorf("$replace reference not found (%w)", ErrInvalidDirective)
)
