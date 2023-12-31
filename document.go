package bkl

import (
	"go.jetpack.io/typeid"
)

type DocPrefix struct{}

func (DocPrefix) Prefix() string { return "doc" }

type DocID struct {
	typeid.TypeID[DocPrefix]
}

type Document struct {
	ID      DocID
	Parents []*Document
	Data    any
}

func NewDocument() *Document {
	return &Document{
		ID: typeid.Must(typeid.New[DocID]()),
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

func (d *Document) AllParents() map[DocID]*Document {
	parents := map[DocID]*Document{}
	d.allParents(parents)
	return parents
}

func (d *Document) allParents(parents map[DocID]*Document) {
	for _, parent := range d.Parents {
		parents[parent.ID] = parent

		for _, doc := range parent.AllParents() {
			parents[doc.ID] = doc
		}
	}
}

func (d *Document) DataAsMap() map[string]any {
	dataMap, ok := d.Data.(map[string]any)
	if ok {
		return dataMap
	} else {
		return nil
	}
}

func (d *Document) PopMapValue(key string) (bool, any) {
	dataMap := d.DataAsMap()
	if dataMap == nil {
		return false, nil
	}

	found, val, data := popMapValue(dataMap, key)

	if found {
		d.Data = data
	}

	return found, val
}

func (d *Document) String() string {
	return d.ID.String()
}
