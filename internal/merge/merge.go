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

// Document applies the supplied document to the current
// internal document state using bkl's merge semantics.
// It returns the updated document slice.
func Document(docs []*document.Document, patch *document.Document) ([]*document.Document, error) {
	matched, updatedDocs, err := patchMatch(docs, patch)
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
		// Avoid modifying input: callers may reuse the same slice across multiple operations
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

// patchMatch attempts to apply the supplied patch to one or more
// documents specified by $match. It returns matched status, updated docs, and error.
// (false, docs, nil) means no $match directive. Zero matches is an error.
func patchMatch(docs []*document.Document, patch *document.Document) (bool, []*document.Document, error) {
	found, m := patch.PopMapValue("$match")
	if !found {
		return false, docs, nil
	}

	if m == nil {
		// Explicit append - create a new slice
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

func findMatches(docs []*document.Document, doc *document.Document, pat any) []*document.Document {
	ret := []*document.Document{}

	// Try parents, then all docs
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

// Files merges multiple files and returns the result in the specified format.
// If format is empty, it defaults to "json-pretty".
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

	// Finalize outputs (e.g., unescape $$)
	for i, out := range outputs {
		outputs[i] = output.FinalizeOutput(out)
	}

	// Sort outputs by path if requested
	if sortPath != "" {
		sortOutputsByPath(outputs, sortPath)
	}

	return ft.MarshalStream(outputs)
}

// FileObj applies an already-parsed file object into the document state.
// It returns the updated document slice.
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

// sortOutputsByPath sorts the outputs slice by the value at the specified path
func sortOutputsByPath(outputs []any, sortPath string) {
	sort.SliceStable(outputs, func(i, j int) bool {
		valI := pathutil.GetString(outputs[i], sortPath)
		valJ := pathutil.GetString(outputs[j], sortPath)
		return valI < valJ
	})
}

// getPathString retrieves a value from a nested structure and converts it to string
