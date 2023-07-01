package bkl

type Parser struct {
	// current map[string]any
}

func New() *Parser {
	return &Parser{}
}

func NewFromFile(path string) (*Parser, error) {
	p := New()

	return p, nil
}

func (p *Parser) Merge(patch map[string]any) {
}
