package bkl

import "github.com/gopatchy/bkl/internal/document"

func processDocument(d *document.Document, mergeFromDocs []*document.Document, env map[string]string) ([]*document.Document, error) {
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
