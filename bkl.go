// Package bkl implements a layered configuration language parser.
package bkl

// Parser carries state for parse operations with multiple layered inputs.
type Parser struct {
	// current map[string]any
}

// New creates and returns a new [Parser] with an empty starting document.
//
// New always succeeds and returns a Parser instance.
func New() *Parser {
	return &Parser{}
}

// NewFromFile creates a new [Parser] then parses the file at path and all its
// preceding layers.
//
// NewFromFile returns either a valid [Parser] with the results of the
// successful parse operation or an error.
func NewFromFile(path string) (*Parser, error) {
	p := New()

	return p, nil
}

// Merge applies the supplied tree to the [Parser]'s current internal document
// state using bkl's merge semantics.
func (p *Parser) Merge(patch map[string]any) {
}
