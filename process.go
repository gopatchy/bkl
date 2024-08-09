package bkl

func Process(obj any, mergeFrom *Document, mergeFromDocs []*Document) (any, error) {
	var err error

	obj, err = process1(obj, mergeFrom, mergeFromDocs, 0)
	if err != nil {
		return nil, err
	}

	obj, err = process2(obj, mergeFrom, mergeFromDocs, 0)
	if err != nil {
		return nil, err
	}

	return obj, nil
}
