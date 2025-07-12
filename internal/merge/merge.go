package merge

import (
	"fmt"
	"io/fs"
	"sort"

	"github.com/gopatchy/bkl/internal/document"
	"github.com/gopatchy/bkl/internal/file"
	"github.com/gopatchy/bkl/internal/format"
	"github.com/gopatchy/bkl/internal/fsys"
	"github.com/gopatchy/bkl/internal/output"
	"github.com/gopatchy/bkl/internal/pathutil"
	"github.com/gopatchy/bkl/internal/process"
	"github.com/gopatchy/bkl/pkg/errors"
	"github.com/gopatchy/bkl/pkg/log"
)

func Document(docs []*document.Document, patch *document.Document) ([]*document.Document, error) {
	matched, updatedDocs, err := patchMatches(docs, patch)
	if err != nil {
		return nil, err
	}
	if matched {
		return updatedDocs, nil
	}

	matched, updatedDocs, err = patchMatch(docs, patch)
	if err != nil {
		return nil, err
	}
	if matched {
		return updatedDocs, nil
	}

	parents := findParents(docs, patch)
	for _, doc := range parents {
		matched = true
		err = process.MergeDocs(doc, patch)
		if err != nil {
			return nil, err
		}
	}

	if !matched {
		newDocs := make([]*document.Document, len(docs), len(docs)+1)
		copy(newDocs, docs)
		newDocs = append(newDocs, patch)
		return newDocs, nil
	}

	return docs, nil
}

func findParents(docs []*document.Document, patch *document.Document) []*document.Document {
	ret := []*document.Document{}

	parents := patch.AllParents()

	for _, doc := range docs {
		if _, found := parents[doc.ID]; found {
			ret = append(ret, doc)
		}
	}

	return ret
}

func patchMatch(docs []*document.Document, patch *document.Document) (bool, []*document.Document, error) {
	found, m := patch.PopMapValue("$match")
	if !found {
		return false, docs, nil
	}

	if m == nil {
		doc := document.New(fmt.Sprintf("%s|matchnull", patch.ID))
		newDocs := make([]*document.Document, len(docs), len(docs)+1)
		copy(newDocs, docs)
		newDocs = append(newDocs, doc)
		err := process.MergeDocs(doc, patch)
		return true, newDocs, err
	}

	matches := findMatches(docs, patch, m)
	if len(matches) == 0 {
		return true, nil, fmt.Errorf("%#v: %w", m, errors.ErrNoMatchFound)
	}

	for _, doc := range matches {
		err := process.MergeDocs(doc, patch)
		if err != nil {
			return true, nil, err
		}
	}

	return true, docs, nil
}

func patchMatches(docs []*document.Document, patch *document.Document) (bool, []*document.Document, error) {
	found, matches := patch.PopMapValue("$matches")
	if !found {
		return false, docs, nil
	}

	matchesList, ok := matches.([]any)
	if !ok {
		return true, nil, fmt.Errorf("$matches must be a list, got %T", matches)
	}

	matchedDocs := make(map[string]*document.Document)

	for i, matchPattern := range matchesList {
		matched := findMatches(docs, patch, matchPattern)
		if len(matched) == 0 {
			return true, nil, fmt.Errorf("$matches[%d] %#v: %w", i, matchPattern, errors.ErrNoMatchFound)
		}

		for _, doc := range matched {
			matchedDocs[doc.ID] = doc
		}
	}

	for _, doc := range matchedDocs {
		err := process.MergeDocs(doc, patch)
		if err != nil {
			return true, nil, err
		}
	}

	return true, docs, nil
}

func findMatches(docs []*document.Document, doc *document.Document, pat any) []*document.Document {
	ret := []*document.Document{}

	parents := findParents(docs, doc)
	for _, ds := range [][]*document.Document{parents, docs} {
		for _, d := range ds {
			if process.MatchDoc(d, pat) {
				ret = append(ret, d)
			}
		}

		if len(ret) > 0 {
			return ret
		}
	}

	return nil
}

func Files(fx fs.FS, files []string, ft *format.Format, env map[string]string, sortPath string) ([]byte, error) {
	var docs []*document.Document
	var deferredDocs []*document.Document
	fileSystem := fsys.New(fx)

	for _, path := range files {
		fileObjs, err := file.LoadAndParents(fileSystem, path, nil)
		if err != nil {
			return nil, err
		}

		for _, f := range fileObjs {
			regularDocs := []*document.Document{}

			for _, doc := range f.Docs {
				deferred := doc.PopMapBoolValue("$defer", true)
				if !deferred {
					deferred, _ = doc.PopListMapBoolValue("$defer", true)
				}

				if deferred {
					deferredDocs = append(deferredDocs, doc)
				} else {
					regularDocs = append(regularDocs, doc)
				}
			}

			docs, err = FileObj(docs, &file.File{
				ID:    f.ID,
				Child: f.Child,
				Path:  f.Path,
				Docs:  regularDocs,
			})
			if err != nil {
				return nil, err
			}
		}
	}

	for _, deferredDoc := range deferredDocs {
		outputs, err := output.Documents(docs, env)
		if err != nil {
			return nil, err
		}

		processedDocs := []*document.Document{}
		for i, out := range outputs {
			doc := document.NewWithData(fmt.Sprintf("output|%d", i), out)
			processedDocs = append(processedDocs, doc)
		}

		docs, err = Document(processedDocs, deferredDoc)
		if err != nil {
			return nil, err
		}
	}

	outputs, err := output.Documents(docs, env)
	if err != nil {
		return nil, err
	}

	for i, out := range outputs {
		outputs[i] = output.FinalizeOutput(out)
	}

	if err := sortOutputsByPath(outputs, sortPath); err != nil {
		return nil, err
	}

	return ft.MarshalStream(outputs)
}

func FileObj(docs []*document.Document, f *file.File) ([]*document.Document, error) {
	log.Debugf("[%s] merging", f)

	for _, doc := range f.Docs {
		log.Debugf("[%s] merging", doc)

		var err error
		docs, err = Document(docs, doc)
		if err != nil {
			return nil, fmt.Errorf("[%s:%s]: %w", f, doc, err)
		}
	}

	return docs, nil
}

func sortOutputsByPath(outputs []any, sortPath string) error {
	var sortErr error
	sort.SliceStable(outputs, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		valI, errI := pathutil.GetString(outputs[i], sortPath)
		if errI != nil {
			sortErr = fmt.Errorf("failed to get sort key at path %q from document %d: %w", sortPath, i, errI)
			return false
		}
		valJ, errJ := pathutil.GetString(outputs[j], sortPath)
		if errJ != nil {
			sortErr = fmt.Errorf("failed to get sort key at path %q from document %d: %w", sortPath, j, errJ)
			return false
		}
		return valI < valJ
	})
	return sortErr
}
