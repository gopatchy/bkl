package bkl

import "github.com/gopatchy/bkl/pkg/errors"

// Re-export all errors from pkg/errors for backward compatibility
var (
	Err = errors.Err

	ErrCircularRef       = errors.ErrCircularRef
	ErrConflictingParent = errors.ErrConflictingParent
	ErrExtraEntries      = errors.ErrExtraEntries
	ErrExtraKeys         = errors.ErrExtraKeys
	ErrInvalidArguments  = errors.ErrInvalidArguments
	ErrInvalidDirective  = errors.ErrInvalidDirective
	ErrInvalidIndex      = errors.ErrInvalidIndex
	ErrInvalidFilename   = errors.ErrInvalidFilename
	ErrInvalidInput      = errors.ErrInvalidInput
	ErrInvalidType       = errors.ErrInvalidType
	ErrInvalidParent     = errors.ErrInvalidParent
	ErrInvalidRepeat     = errors.ErrInvalidRepeat
	ErrMarshal           = errors.ErrMarshal
	ErrRefNotFound       = errors.ErrRefNotFound
	ErrMissingEnv        = errors.ErrMissingEnv
	ErrMissingFile       = errors.ErrMissingFile
	ErrMissingMatch      = errors.ErrMissingMatch
	ErrMultiMatch        = errors.ErrMultiMatch
	ErrNoMatchFound      = errors.ErrNoMatchFound
	ErrNoCloneFound      = errors.ErrNoCloneFound
	ErrOutputFile        = errors.ErrOutputFile
	ErrRequiredField     = errors.ErrRequiredField
	ErrUnknownFormat     = errors.ErrUnknownFormat
	ErrUnmarshal         = errors.ErrUnmarshal
	ErrUselessOverride   = errors.ErrUselessOverride
	ErrVariableNotFound  = errors.ErrVariableNotFound
)
