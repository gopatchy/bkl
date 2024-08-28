package bkl

import "fmt"

var (
	// Base error; every error in bkl inherits from this
	Err = fmt.Errorf("bkl error")

	// Format and system errors
	ErrCircularRef        = fmt.Errorf("circular reference (%w)", Err)
	ErrConflictingParent  = fmt.Errorf("conflicting $parent (%w)", Err)
	ErrExtraEntries       = fmt.Errorf("extra entries (%w)", Err)
	ErrExtraKeys          = fmt.Errorf("extra keys (%w)", Err)
	ErrInvalidArguments   = fmt.Errorf("invalid arguments (%w)", Err)
	ErrInvalidDirective   = fmt.Errorf("invalid directive (%w)", Err)
	ErrInvalidIndex       = fmt.Errorf("invalid index (%w)", Err)
	ErrInvalidFilename    = fmt.Errorf("invalid filename (%w)", Err)
	ErrInvalidType        = fmt.Errorf("invalid type (%w)", Err)
	ErrInvalidParent      = fmt.Errorf("invalid $parent (%w)", Err)
	ErrInvalidRepeat      = fmt.Errorf("invalid $repeat (%w)", Err)
	ErrInvalidRepeatCount = fmt.Errorf("invalid $repeat.$count (%w)", Err)
	ErrInvalidRepeatName  = fmt.Errorf("invalid $repeat.$name (%w)", Err)
	ErrMarshal            = fmt.Errorf("encoding error (%w)", Err)
	ErrRefNotFound        = fmt.Errorf("reference not found (%w)", Err)
	ErrMissingEnv         = fmt.Errorf("missing environment variable (%w)", Err)
	ErrMissingFile        = fmt.Errorf("missing file (%w)", Err)
	ErrMissingMatch       = fmt.Errorf("missing $match (%w)", Err)
	ErrMissingRepeatCount = fmt.Errorf("missing $repeat.$count (%w)", Err)
	ErrMissingRepeatName  = fmt.Errorf("missing $repeat.$name (%w)", Err)
	ErrMultiMatch         = fmt.Errorf("multiple documents $match (%w)", Err)
	ErrNoMatchFound       = fmt.Errorf("no document/entry matched $match (%w)", Err)
	ErrNoCloneFound       = fmt.Errorf("no document/entry matched $clone (%w)", Err)
	ErrOutputFile         = fmt.Errorf("error opening output file (%w)", Err)
	ErrRequiredField      = fmt.Errorf("required field not set (%w)", Err)
	ErrUnknownFormat      = fmt.Errorf("unknown format (%w)", Err)
	ErrUnmarshal          = fmt.Errorf("decoding error (%w)", Err)
	ErrUselessOverride    = fmt.Errorf("useless override (%w)", Err)
	ErrVariableNotFound   = fmt.Errorf("variable not found (%w)", Err)
)
