package bkl

import (
	"fmt"
)

type document struct {
	ID      string
	Parents []*document
	Data    any
}

func newDocument(id string) *document {
	return &document{
		ID: id,
	}
}

func newDocumentWithData(id string, data any) *document {
	doc := newDocument(id)
	doc.Data = data
	return doc
}

func (d *document) allParents() map[string]*document {
	parents := map[string]*document{}
	d.allParentsInt(parents)
	return parents
}

func (d *document) allParentsInt(parents map[string]*document) {
	for _, parent := range d.Parents {
		parents[parent.ID] = parent

		for _, doc := range parent.allParents() {
			parents[doc.ID] = doc
		}
	}
}

func (d *document) clone(suffix string) (*document, error) {
	data, err := deepClone(d.Data)
	if err != nil {
		return nil, err
	}

	d2 := newDocumentWithData(fmt.Sprintf("%s|%s", d, suffix), data)

	for _, parent := range d.Parents {
		d2.Parents = append(d2.Parents, parent)
	}

	return d2, nil
}

func (d *document) popMapValue(key string) (bool, any) {
	dataMap, ok := d.Data.(map[string]any)
	if !ok {
		return false, nil
	}

	found, val, data := popMapValue(dataMap, key)

	if found {
		d.Data = data
	}

	return found, val
}

func (d *document) process(mergeFromDocs []*document, env map[string]string) ([]*document, error) {
	var err error

	ec := newEvalContext(env)

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

func (d *document) String() string {
	return d.ID
}
