package bkl

import (
	"go.jetpack.io/typeid"
)

type Document struct {
	ID      typeid.TypeID
	Parents []*Document
	Data    any
}

func NewDocument() *Document {
	return &Document{
		ID: typeid.Must(typeid.New("doc")),
	}
}

func (d *Document) String() string {
	return d.ID.String()
}
