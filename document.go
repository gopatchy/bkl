package bkl

import (
	"go.jetpack.io/typeid"
)

type document struct {
	id      typeid.TypeID
	parents []*document
	data    any
}

func newDocument() *document {
	return &document{
		id: typeid.Must(typeid.New("doc")),
	}
}

func (d *document) String() string {
	return d.id.String()
}
