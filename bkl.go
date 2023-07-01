package bkl

type Parser struct {
}

func New() *Parser {
	return &Parser{}
}

func NewFromFile(path string) (*Parser, error) {
	p := New()

	return p, nil
}
