package document

import (
	"fmt"

	"github.com/gopatchy/bkl/internal/utils"
)

type Document struct {
	ID      string
	Parents []*Document
	Data    any
}

func New(id string) *Document {
	return &Document{
		ID: id,
	}
}

func NewWithData(id string, data any) *Document {
	doc := New(id)
	doc.Data = data
	return doc
}

func (d *Document) AllParents() map[string]*Document {
	parents := map[string]*Document{}
	d.allParentsInt(parents)
	return parents
}

func (d *Document) allParentsInt(parents map[string]*Document) {
	for _, parent := range d.Parents {
		parents[parent.ID] = parent

		for _, doc := range parent.AllParents() {
			parents[doc.ID] = doc
		}
	}
}

func (d *Document) Clone(suffix string) (*Document, error) {
	data, err := utils.DeepClone(d.Data)
	if err != nil {
		return nil, err
	}

	d2 := NewWithData(fmt.Sprintf("%s|%s", d, suffix), data)

	for _, parent := range d.Parents {
		d2.Parents = append(d2.Parents, parent)
	}

	return d2, nil
}

func (d *Document) PopMapValue(key string) (bool, any) {
	dataMap, ok := d.Data.(map[string]any)
	if !ok {
		return false, nil
	}

	found, val, data := utils.PopMapValue(dataMap, key)

	if found {
		d.Data = data
	}

	return found, val
}

func (d *Document) PopMapBoolValue(key string, val bool) bool {
	dataMap, ok := d.Data.(map[string]any)
	if !ok {
		return false
	}

	found, data := utils.PopMapBoolValue(dataMap, key, val)

	if found {
		d.Data = data
	}

	return found
}

func (d *Document) PopListMapBoolValue(key string, val bool) (bool, error) {
	dataList, ok := d.Data.([]any)
	if !ok {
		return false, nil
	}

	found, data, err := utils.PopListMapBoolValue(dataList, key, val)
	if err != nil {
		return false, err
	}

	if found {
		d.Data = data
	}

	return found, nil
}

func (d *Document) String() string {
	return d.ID
}
