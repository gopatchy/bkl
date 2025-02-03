package bkl

import (
	"fmt"
)

type Document struct {
	ID      string
	Parents []*Document
	Data    any
}

func NewDocument(id string) *Document {
	return &Document{
		ID: id,
	}
}

func NewDocumentWithData(id string, data any) *Document {
	doc := NewDocument(id)
	doc.Data = data
	return doc
}

func (d *Document) AddParents(parents ...*Document) {
	d.Parents = append(d.Parents, parents...)
}

func (d *Document) AllParents() map[string]*Document {
	parents := map[string]*Document{}
	d.allParents(parents)
	return parents
}

func (d *Document) allParents(parents map[string]*Document) {
	for _, parent := range d.Parents {
		parents[parent.ID] = parent

		for _, doc := range parent.AllParents() {
			parents[doc.ID] = doc
		}
	}
}

func (d *Document) Clone(suffix string) (*Document, error) {
	data, err := deepClone(d.Data)
	if err != nil {
		return nil, err
	}

	d2 := NewDocumentWithData(fmt.Sprintf("%s|%s", d, suffix), data)

	for _, parent := range d.Parents {
		d2.Parents = append(d2.Parents, parent)
	}

	return d2, nil
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

func (d *Document) Process(mergeFromDocs []*Document) ([]*Document, error) {
	var err error

	ec := NewEvalContext()

	d.Data, err = process1(d.Data, d, mergeFromDocs, 0)
	if err != nil {
		return nil, err
	}

	docs, ecs, err := repeatDoc(d, ec)
	if err != nil {
		return nil, err
	}

	for i, doc := range docs {
		doc.Data, err = process2(doc.Data, doc, mergeFromDocs, ecs[i], 0)
		if err != nil {
			return nil, err
		}
	}

	return docs, nil
}

func (d *Document) String() string {
	return d.ID
}
