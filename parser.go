// Package bkl implements a layered configuration language parser.
//
//   - Language & tool documentation: https://bkl.gopatchy.io/
//   - Go library source: https://github.com/gopatchy/bkl
//   - Go library documentation: https://pkg.go.dev/github.com/gopatchy/bkl
package bkl

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
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
// Phase 1
//   - $parent
//
// Phase 2
//   - $delete
//   - $replace: true
//
// Phase 3
//   - $merge
//   - $replace: map
//   - $replace: string
//
// Phase 4
//   - $repeat: int
//
// Phase 5
//   - $""
//   - $encode
//   - $decode
//   - $env
//   - $repeat
//   - $value
//
// Phase 6
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
	docs  []*Document
	fsys  *FS
	debug bool
}

func New() (*Parser, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return NewWithPath("/", wd)
}

func NewWithPath(path string, wd string) (*Parser, error) {
	return NewWithFS(os.DirFS(path), wd)
}

func NewWithFS(fsys fs.FS, wd string) (*Parser, error) {
	return &Parser{
		fsys:  NewFS(fsys, wd),
		debug: os.Getenv("BKL_DEBUG") != "",
	}, nil
}

// SetDebug enables or disables debug log output to stderr.
func (p *Parser) SetDebug(debug bool) {
	p.debug = debug
}

// MergeDocument applies the supplied Document to the [Parser]'s current
// internal document state using bkl's merge semantics. If expand is true,
// documents without $match will append; otherwise this is an error.
func (p *Parser) MergeDocument(patch *Document) error {
	matched, err := p.mergePatchMatch(patch)
	if err != nil {
		return err
	}

	if matched {
		return nil
	}

	for _, doc := range p.parents(patch) {
		matched = true

		err = mergeDocs(doc, patch)
		if err != nil {
			return err
		}
	}

	if !matched {
		p.docs = append(p.docs, patch)
	}

	return nil
}

func (p *Parser) parents(patch *Document) []*Document {
	ret := []*Document{}

	parents := patch.AllParents()

	for _, doc := range p.docs {
		if _, found := parents[doc.ID]; found {
			ret = append(ret, doc)
		}
	}

	return ret
}

// mergePatchMatch attempts to apply the supplied patch to one or more
// documents specified by $match. It returns success and error separately;
// (false, nil) means no $match directive. Zero matches is an error.
func (p *Parser) mergePatchMatch(patch *Document) (bool, error) {
	found, m := patch.PopMapValue("$match")
	if !found {
		return false, nil
	}

	if m == nil {
		// Explicit append
		doc := NewDocument(fmt.Sprintf("%s|matchnull", patch.ID))
		p.docs = append(p.docs, doc)
		return true, mergeDocs(doc, patch)
	}

	docs := p.findMatches(patch, m)
	if len(docs) == 0 {
		return true, fmt.Errorf("%#v: %w", m, ErrNoMatchFound)
	}

	for _, doc := range docs {
		err := mergeDocs(doc, patch)
		if err != nil {
			return true, err
		}
	}

	return true, nil
}

func (p *Parser) findMatches(doc *Document, pat any) []*Document {
	ret := []*Document{}

	// Try parents, then all docs
	for _, ds := range [][]*Document{p.parents(doc), p.docs} {
		for _, d := range ds {
			if matchDoc(d, pat) {
				ret = append(ret, d)
			}
		}

		if len(ret) > 0 {
			return ret
		}
	}

	return nil
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
	p.log("[%s] merging", f)

	for _, doc := range f.docs {
		p.log("[%s] merging", doc)

		err := p.MergeDocument(doc)
		if err != nil {
			return fmt.Errorf("[%s:%s]: %w", f, doc, err)
		}
	}

	return nil
}

// Documents returns the parsed, merged (but not processed) trees for all
// documents.
func (p *Parser) Documents() []*Document {
	return p.docs
}

// outputDocument returns the output objects generated by the specified
// document.
func (p *Parser) outputDocument(doc *Document) ([]any, error) {
	docs, err := doc.Process(p.docs)
	if err != nil {
		return nil, err
	}

	outs := []any{}

	for _, d := range docs {
		obj, out, err := findOutputs(d.Data)
		if err != nil {
			return nil, err
		}

		if len(out) == 0 {
			outs = append(outs, obj)
		} else {
			outs = append(outs, out...)
		}
	}

	return filterList(outs, func(v any) ([]any, error) {
		v2, include, err := filterOutput(v)
		if err != nil {
			return nil, err
		}

		if !include {
			return nil, nil
		}

		err = validate(v2)
		if err != nil {
			return nil, err
		}

		return []any{finalizeOutput(v2)}, nil
	})
}

// OutputDocuments returns the output objects generated by all documents.
func (p *Parser) OutputDocuments() ([]any, error) {
	ret := []any{}

	for _, doc := range p.docs {
		outs, err := p.outputDocument(doc)
		if err != nil {
			return nil, err
		}

		ret = append(ret, outs...)
	}

	return ret, nil
}

// Output returns all documents encoded in the specified format and merged into
// a stream.
func (p *Parser) Output(format string) ([]byte, error) {
	outs, err := p.OutputDocuments()
	if err != nil {
		return nil, err
	}

	f, err := GetFormat(format)
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
		return errors.Join(fmt.Errorf("%s: %w", path, ErrOutputFile), err)
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
		return errors.Join(ErrOutputFile, err)
	}

	return nil
}

func (p *Parser) log(format string, v ...any) {
	if !p.debug {
		return
	}

	log.Printf(format, v...)
}

// FileMatch attempts to find a file with the same base name as path, but
// possibly with a different supported extension. It is intended to support
// "virtual" filenames that auto-convert from the format of the underlying
// real file.
//
// Returns the real filename and the requested output format, or
// ("", "", error).
func (p *Parser) FileMatch(path string) (string, string, error) {
	format := ext(path)
	if _, found := formatByExtension[format]; !found {
		return "", "", fmt.Errorf("%s: %w", format, ErrUnknownFormat)
	}

	withoutExt := strings.TrimSuffix(path, "."+format)

	if filepath.Base(withoutExt) == "-" {
		return path, format, nil
	}

	realPath := p.fsys.findFile(withoutExt)

	if realPath == "" {
		return "", "", fmt.Errorf("%s.*: %w", withoutExt, ErrMissingFile)
	}

	return realPath, format, nil
}
