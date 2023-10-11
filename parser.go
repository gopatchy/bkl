// Package bkl implements a layered configuration language parser.
//
//   - Language & tool documentation: https://bkl.gopatchy.io/
//   - Go library source: https://github.com/gopatchy/bkl
//   - Go library documentation: https://pkg.go.dev/github.com/gopatchy/bkl
package bkl

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/gopatchy/bkl/polyfill"
)

// A Parser reads input documents, merges layers, and generates outputs.
//
// # Terminology
//   - Each Parser can read multiple files
//   - Each file represents a single layer
//   - Each file contains one or more documents
//   - Each document generates one or more outputs
//
// # Directive Evaluation Order
//
// Directive evaluation order can matter, e.g. if you $merge a subtree that
// contains an $output directive.
//
// Merge phase 1 (load)
//   - $parent
//
// Merge phase 2 (evaluate)
//   - $env
//
// Merge phase 3 (merge)
//   - $delete
//   - $replace: true
//
// Output phase 1 (process)
//   - $merge
//   - $replace: map
//   - $replace: string
//   - $encode
//
// Output phase 2 (output)
//   - $output
//
// # Document Layer Matching Logic
//
// When applying a new document to internal state, it may be merged into one or
// more existing documents or appended as a new document. To select merge
// targets, Parser considers (in order):
//   - If $match:
//   - $match: null -> append
//   - $match within parent documents -> merge
//   - $match any documents -> merge
//   - No matching documents -> error
//   - If parent documents -> merge into all parents
//   - If no parent documents -> append
type Parser struct {
	docs  []*document
	debug bool
}

// New creates and returns a new [Parser] with an empty starting document set.
//
// New always succeeds and returns a Parser instance.
func New() *Parser {
	return &Parser{}
}

// SetDebug enables or disables debug log output to stderr.
func (p *Parser) SetDebug(debug bool) {
	p.debug = debug
}

// MergePatch applies the supplied patch to the [Parser]'s current internal
// document state using bkl's merge semantics. If expand is true, documents
// without $match will append; otherwise this is an error.
func (p *Parser) MergePatch(patch any, expand bool) error {
	matched, err := p.mergePatchMatch(patch)
	if err != nil {
		return err
	}

	if matched {
		return nil
	}

	if expand {
		p.docs = append(p.docs, newDocument())
		return p.mergePatch(patch, len(p.docs)-1)
	}

	// Don't require $match when there's only one document
	if len(p.docs) == 1 {
		return p.mergePatch(patch, 0)
	}

	return fmt.Errorf("%v: %w", patch, ErrMissingMatch)
}

// mergePatch applies the supplied patch to the document at the specified
// index.
func (p *Parser) mergePatch(patch any, index int) error {
	merged, err := merge(p.docs[index].data, patch)
	if err != nil {
		return err
	}

	p.docs[index].data = merged

	return nil
}

// mergePatchMatch attempts to apply the supplied patch to one or more
// documents specified by $match. It returns success and error separately;
// (false, nil) means no $match directive. Zero matches is an error.
func (p *Parser) mergePatchMatch(patch any) (bool, error) {
	patchMap, ok := patch.(map[string]any)
	if !ok {
		return false, nil
	}

	found, m, patch := popMapValue(patchMap, "$match")
	if !found {
		return false, nil
	}

	if m == nil {
		// Explicit append
		p.docs = append(p.docs, newDocument())
		return true, p.mergePatch(patch, len(p.docs)-1)
	}

	// Must find at least one match
	found = false

	for i, doc := range p.docs {
		if match(doc.data, m) {
			err := p.mergePatch(patch, i)
			if err != nil {
				return true, err
			}

			found = true
		}
	}

	if !found {
		return true, fmt.Errorf("%#v: %w", m, ErrNoMatchFound)
	}

	return true, nil
}

// MergeFile parses the file at path and merges its contents into the
// [Parser]'s document state using bkl's merge semantics.
func (p *Parser) MergeFile(path string) error {
	f, err := p.loadFile(path, nil)
	if err != nil {
		return err
	}

	return p.mergeFile(f)
}

// MergeFileLayers determines relevant layers from the supplied path and merges
// them in order.
func (p *Parser) MergeFileLayers(path string) error {
	files, err := p.loadFileAndParents(path, nil)
	if err != nil {
		return err
	}

	for _, f := range files {
		err := p.mergeFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

// mergeFile applies an already-parsed file object into the [Parser]'s
// document state.
func (p *Parser) mergeFile(f *file) error {
	// First file in an empty parser can append without $match
	first := len(p.docs) == 0

	for i, doc := range f.docs {
		p.log("[%s] merging", f.path)

		err := p.MergePatch(doc, first)
		if err != nil {
			return fmt.Errorf("[%s:doc%d]: %w", f.path, i, err)
		}
	}

	return nil
}

// NumDocuments returns the number of documents in the [Parser]'s internal
// state.
func (p *Parser) NumDocuments() int {
	return len(p.docs)
}

// Document returns the parsed, merged (but not processed) tree for the
// document at index.
func (p *Parser) Document(index int) (any, error) {
	if index >= p.NumDocuments() {
		return nil, fmt.Errorf("%d: %w", index, ErrInvalidIndex)
	}

	return p.docs[index].data, nil
}

// Documents returns the parsed, merged (but not processed) trees for all
// documents.
func (p *Parser) Documents() ([]any, error) {
	ret := []any{}

	for _, doc := range p.docs {
		ret = append(ret, doc.data)
	}

	return ret, nil
}

// OutputDocumentsIndex returns the output objects generated by the document
// at the specified index.
func (p *Parser) OutputDocumentsIndex(index int) ([]any, error) {
	obj, err := p.Document(index)
	if err != nil {
		return nil, err
	}

	docs, err := p.Documents()
	if err != nil {
		return nil, err
	}

	obj, err = Process(obj, obj, docs)
	if err != nil {
		return nil, err
	}

	if obj == nil {
		return nil, nil
	}

	obj, outs, err := findOutputs(obj)
	if err != nil {
		return nil, err
	}

	if len(outs) == 0 {
		outs = append(outs, obj)
	}

	outs, err = filterList(outs, func(v any) ([]any, error) {
		v2, err := filterOutput(v)
		if err != nil {
			return nil, err
		}

		if v2 == nil {
			return nil, nil
		}

		err = validate(v2)
		if err != nil {
			return nil, err
		}

		return []any{v2}, nil
	})

	if err != nil {
		return nil, err
	}

	return outs, nil
}

// OutputDocuments returns the output objects generated by all documents.
func (p *Parser) OutputDocuments() ([]any, error) {
	ret := []any{}

	for i := 0; i < p.NumDocuments(); i++ {
		outs, err := p.OutputDocumentsIndex(i)
		if err != nil {
			return nil, err
		}

		ret = append(ret, outs...)
	}

	return ret, nil
}

// Output returns all documents encoded in the specified format and merged into
// a stream.
func (p *Parser) Output(ext string) ([]byte, error) {
	outs, err := p.OutputDocuments()
	if err != nil {
		return nil, err
	}

	f, err := GetFormat(ext)
	if err != nil {
		return nil, err
	}

	return f.MarshalStream(outs)
}

// OutputToFile encodes all documents in the specified format and writes them
// to the specified output path.
//
// If format is "", it is inferred from path's file extension.
func (p *Parser) OutputToFile(path, format string) error {
	if format == "" {
		format = ext(path)
	}

	fh, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return polyfill.ErrorsJoin(fmt.Errorf("%s: %w", path, ErrOutputFile), err)
	}

	defer fh.Close()

	err = p.OutputToWriter(fh, format)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return nil
}

// OutputToWriter encodes all documents in the specified format and writes them
// to the specified [io.Writer].
//
// If format is "", it defaults to "json-pretty".
func (p *Parser) OutputToWriter(fh io.Writer, format string) error {
	if format == "" {
		format = "json-pretty"
	}

	out, err := p.Output(format)
	if err != nil {
		return err
	}

	_, err = fh.Write(out)
	if err != nil {
		return polyfill.ErrorsJoin(ErrOutputFile, err)
	}

	return nil
}

func (p *Parser) log(format string, v ...any) {
	if !p.debug {
		return
	}

	log.Printf(format, v...)
}
