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

func NewDocumentWithData(data any) *Document {
	doc := NewDocument()
	doc.Data = data
	return doc
}

func (d *Document) AddParents(parents ...*Document) {
	d.Parents = append(d.Parents, parents...)
}

func (d *Document) AllParents() []*Document {
	ret := append([]*Document{}, d.Parents...)

	for _, parent := range d.Parents {
		ret = append(ret, parent.AllParents()...)
	}

	return ret
}

func (d *Document) String() string {
	return d.ID.String()
}
