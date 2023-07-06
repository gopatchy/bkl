// Package bkl implements a layered configuration language parser.
package bkl

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
)

var (
	// Base error; every error in bkl inherits from this
	Err = fmt.Errorf("bkl error")

	// Format and system errors
	ErrEncode          = fmt.Errorf("encoding error (%w)", Err)
	ErrDecode          = fmt.Errorf("decoding error (%w)", Err)
	ErrInvalidIndex    = fmt.Errorf("invalid index (%w)", Err)
	ErrInvalidFilename = fmt.Errorf("invalid filename (%w)", Err)
	ErrInvalidType     = fmt.Errorf("invalid type (%w)", Err)
	ErrMissingFile     = fmt.Errorf("missing file (%w)", Err)
	ErrNoMatchFound    = fmt.Errorf("no document matched $match (%w)", Err)
	ErrRequiredField   = fmt.Errorf("required field not set (%w)", Err)
	ErrUnknownFormat   = fmt.Errorf("unknown format (%w)", Err)

	// Base language directive error
	ErrInvalidDirective = fmt.Errorf("invalid directive (%w)", Err)

	// Specific language directive errors
	ErrInvalidMergeType   = fmt.Errorf("invalid $merge type (%w)", ErrInvalidDirective)
	ErrInvalidParentType  = fmt.Errorf("invalid $parent type (%w)", ErrInvalidDirective)
	ErrInvalidPatchType   = fmt.Errorf("invalid $patch type (%w)", ErrInvalidDirective)
	ErrInvalidPatchValue  = fmt.Errorf("invalid $patch value (%w)", ErrInvalidDirective)
	ErrInvalidReplaceType = fmt.Errorf("invalid $replace type (%w)", ErrInvalidDirective)
	ErrMergeRefNotFound   = fmt.Errorf("$merge reference not found (%w)", ErrInvalidDirective)
	ErrReplaceRefNotFound = fmt.Errorf("$replace reference not found (%w)", ErrInvalidDirective)

	baseTemplate = ""
)

// Parser carries state for parse operations with multiple layered inputs.
type Parser struct {
	docs  []any
	debug bool
}

// New creates and returns a new [Parser] with an empty starting document.
//
// New always succeeds and returns a Parser instance.
func New() *Parser {
	return &Parser{}
}

// NewFromFile creates a new [Parser] then calls [MergeFileLayers()] with
// the supplied path.
func NewFromFile(path string) (*Parser, error) {
	p := New()

	err := p.MergeFileLayers(path)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Parser) SetDebug(debug bool) {
	p.debug = debug
}

// MergeOther applies other's internal document state to ours, using bkl's
// merge semantics.
func (p *Parser) MergeOther(other *Parser) error {
	for i, doc := range other.docs {
		err := p.MergePatch(i, doc)
		if err != nil {
			return err
		}
	}

	return nil
}

// MergePatch applies the supplied patch to the [Parser]'s current internal
// document state (at the specified document index) using bkl's merge
// semantics.
func (p *Parser) MergePatch(index int, patch any) error {
	if index >= len(p.docs) {
		p.docs = append(p.docs, make([]any, index-len(p.docs)+1)...)
	}

	merged, err := merge(p.docs[index], patch)
	if err != nil {
		return err
	}

	p.docs[index] = merged

	return nil
}

// MergeIndexBytes parses the supplied doc bytes as the format specified by ext
// (file extension), then calls [MergePatch()].
//
// index is taken as a hint but can be overridden by $match.
func (p *Parser) MergeIndexBytes(index int, doc []byte, ext string) error {
	f, found := formatByExtension[ext]
	if !found {
		return fmt.Errorf("%s: %w", ext, ErrUnknownFormat)
	}

	patch, err := f.decode(doc)
	if err != nil {
		return fmt.Errorf("%w / %w", err, ErrDecode)
	}

	if patchMap, ok := canonicalizeType(patch).(map[string]any); ok {
		m, found := patchMap["$match"]
		if found {
			delete(patchMap, "$match")

			index = -1

			for i, doc := range p.docs {
				if match(doc, m) {
					index = i
					break
				}
			}

			if index == -1 {
				return fmt.Errorf("%#v: %w", m, ErrNoMatchFound)
			}
		}
	}

	err = p.MergePatch(index, patch)
	if err != nil {
		return err
	}

	return nil
}

// MergeMultiBytes calls [MergeIndexBytes()] once for each item in the outer
// slice.
func (p *Parser) MergeMultiBytes(bs [][]byte, ext string) error {
	for i, b := range bs {
		err := p.MergeIndexBytes(i, b, ext)
		if err != nil {
			return fmt.Errorf("index %d (of [0,%d]): %w", i, len(bs)-1, err)
		}
	}

	return nil
}

// MergeBytes splits its input into multiple documents (using the ---
// delimiter) then calls [MergeMultiBytes()].
func (p *Parser) MergeBytes(b []byte, ext string) error {
	docs := bytes.SplitAfter(b, []byte("\n---\n"))

	for i, doc := range docs {
		// Leave the initial \n attached
		docs[i] = bytes.TrimSuffix(doc, []byte("---\n"))
	}

	return p.MergeMultiBytes(docs, ext)
}

// MergeReader reads all input then calls [MergeBytes()].
func (p *Parser) MergeReader(in io.Reader, ext string) error {
	b, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	return p.MergeBytes(b, ext)
}

// MergeFile opens the supplied path and determines the file format from the
// file extension, then calls [MergeReader()].
func (p *Parser) MergeFile(path string) error {
	p.log("loading %s", path)

	fh, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	defer fh.Close()

	err = p.MergeReader(fh, Ext(path))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return nil
}

// MergeFileLayers determines relevant layers from the supplied path and merges
// them in order.
func (p *Parser) MergeFileLayers(path string) error {
	paths := []string{
		path,
	}

	for {
		parent, err := getParent(path)
		if err != nil {
			return err
		}

		if *parent == baseTemplate {
			break
		}

		path = *parent
		paths = append(paths, path)
	}

	slices.Reverse(paths)

	for _, path := range paths {
		err := p.MergeFile(path)
		if err != nil {
			return err
		}
	}

	return nil
}

// Count returns the number of documents.
func (p *Parser) Count() int {
	return len(p.docs)
}

// GetIndex returns the parsed tree for the document at index.
func (p *Parser) GetIndex(index int) (any, error) {
	if index >= p.Count() {
		return nil, fmt.Errorf("%d: %w", index, ErrInvalidIndex)
	}

	return p.docs[index], nil
}

// GetOutputIndex returns the document at index, encoded as ext.
func (p *Parser) GetOutputIndex(index int, ext string) ([][]byte, error) {
	obj, err := p.GetIndex(index)
	if err != nil {
		return nil, err
	}

	obj, err = postMerge(obj)
	if err != nil {
		return nil, err
	}

	err = validate(obj)
	if err != nil {
		return nil, err
	}

	outs := findOutputs(obj)
	if len(outs) == 0 {
		outs = append(outs, obj)
	}

	f, found := formatByExtension[ext]
	if !found {
		return nil, fmt.Errorf("%s: %w", ext, ErrUnknownFormat)
	}

	encs := [][]byte{}

	for _, out := range outs {
		enc, err := f.encode(out)
		if err != nil {
			return nil, fmt.Errorf("index %d (of [0,%d]): %w (%w)", index, p.Count()-1, err, ErrEncode)
		}

		encs = append(encs, enc)
	}

	return encs, nil
}

// GetOutputLayers returns all layers encoded as ext.
func (p *Parser) GetOutputLayers(ext string) ([][]byte, error) {
	outs := [][]byte{}

	for i := 0; i < p.Count(); i++ {
		out, err := p.GetOutputIndex(i, ext)
		if err != nil {
			return nil, err
		}

		outs = append(outs, out...)
	}

	return outs, nil
}

// GetOutput returns all documents encoded as ext and merged with ---.
func (p *Parser) GetOutput(ext string) ([]byte, error) {
	outs, err := p.GetOutputLayers(ext)
	if err != nil {
		return nil, err
	}

	return bytes.Join(outs, []byte("---\n")), nil
}

func (p *Parser) log(format string, v ...any) {
	if !p.debug {
		return
	}

	log.Printf(format, v...)
}
