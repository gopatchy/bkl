// Package bkl implements a layered configuration language parser.
//
//   - Language & tool documentation: https://bkl.gopatchy.io/
//   - Go library source: https://github.com/gopatchy/bkl
//   - Go library documentation: https://pkg.go.dev/github.com/gopatchy/bkl
package bkl

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopatchy/bkl/internal/document"
	"github.com/gopatchy/bkl/internal/file"
	"github.com/gopatchy/bkl/internal/format"
	"github.com/gopatchy/bkl/internal/fsys"
	"github.com/gopatchy/bkl/internal/output"
	"github.com/gopatchy/bkl/internal/process"
	"github.com/gopatchy/bkl/internal/utils"
	"github.com/gopatchy/bkl/pkg/errors"
	"github.com/gopatchy/bkl/pkg/log"
)

// bkl reads input documents, merges layers, and generates outputs.
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
// # document Layer Matching Logic
//
// When applying a new document to internal state, it may be merged into one or
// more existing documents or appended as a new document. To select merge
// targets, bkl considers (in order):
//   - If $match:
//   - $match: null -> append
//   - $match within parent documents -> merge
//   - $match any documents -> merge
//   - No matching documents -> error
//   - If parent documents -> merge into all parents
//   - If no parent documents -> append

// mergeDocument applies the supplied document to the current
// internal document state using bkl's merge semantics.
// It returns the updated document slice.
func mergeDocument(docs []*document.Document, patch *document.Document) ([]*document.Document, error) {
	matched, updatedDocs, err := mergePatchMatch(docs, patch)
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
		// Create a new slice to avoid modifying the input
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

// mergePatchMatch attempts to apply the supplied patch to one or more
// documents specified by $match. It returns matched status, updated docs, and error.
// (false, docs, nil) means no $match directive. Zero matches is an error.
func mergePatchMatch(docs []*document.Document, patch *document.Document) (bool, []*document.Document, error) {
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

// mergeFiles merges multiple files and returns the result in the specified format.
// If format is empty, it defaults to "json-pretty".
func mergeFiles(fx fs.FS, files []string, ft *format.Format, env map[string]string) ([]byte, error) {
	var docs []*document.Document
	fileSystem := fsys.New(fx)

	for _, path := range files {
		fileObjs, err := file.LoadAndParents(fileSystem, path, nil)
		if err != nil {
			return nil, err
		}

		for _, f := range fileObjs {
			docs, err = mergeFileObj(docs, f)
			if err != nil {
				return nil, err
			}
		}
	}

	return outputBytes(docs, ft, env)
}

// mergeFileObj applies an already-parsed file object into the document state.
// It returns the updated document slice.
func mergeFileObj(docs []*document.Document, f *file.File) ([]*document.Document, error) {
	log.Debugf("[%s] merging", f)

	for _, doc := range f.Docs {
		log.Debugf("[%s] merging", doc)

		var err error
		docs, err = mergeDocument(docs, doc)
		if err != nil {
			return nil, fmt.Errorf("[%s:%s]: %w", f, doc, err)
		}
	}

	return docs, nil
}

// outputDocument returns the output objects generated by the specified
// document.
func outputDocument(docs []*document.Document, doc *document.Document, env map[string]string) ([]any, error) {
	processedDocs, err := process.Document(doc, docs, env)
	if err != nil {
		return nil, err
	}

	outs := []any{}

	for _, d := range processedDocs {
		obj, out, err := output.FindOutputs(d.Data)
		if err != nil {
			return nil, err
		}

		if len(out) == 0 {
			outs = append(outs, obj)
		} else {
			outs = append(outs, out...)
		}
	}

	return utils.FilterList(outs, func(v any) ([]any, error) {
		v2, include, err := output.FilterOutput(v)
		if err != nil {
			return nil, err
		}

		if !include {
			return nil, nil
		}

		err = process.Validate(v2)
		if err != nil {
			return nil, err
		}

		return []any{output.FinalizeOutput(v2)}, nil
	})
}

// outputDocuments returns the output objects generated by all documents.
func outputDocuments(docs []*document.Document, env map[string]string) ([]any, error) {
	ret := []any{}

	for _, doc := range docs {
		outs, err := outputDocument(docs, doc, env)
		if err != nil {
			return nil, err
		}

		ret = append(ret, outs...)
	}

	return ret, nil
}

// outputBytes returns all documents encoded in the specified format and merged into
// a stream.
func outputBytes(docs []*document.Document, ft *format.Format, env map[string]string) ([]byte, error) {
	outs, err := outputDocuments(docs, env)
	if err != nil {
		return nil, err
	}

	return ft.MarshalStream(outs)
}

// makePathsAbsolute converts relative paths to absolute paths using the provided working directory.
func makePathsAbsolute(paths []string, workingDir string) ([]string, error) {
	result := make([]string, len(paths))
	for i, path := range paths {
		if filepath.IsAbs(path) {
			result[i] = path
		} else {
			result[i] = filepath.Join(workingDir, path)
		}
	}
	return result, nil
}

// rebasePathsToRoot rebases absolute paths to be relative to the root path.
func rebasePathsToRoot(absPaths []string, rootPath string, workingDir string) ([]string, error) {
	absRootPath := rootPath
	if !filepath.IsAbs(rootPath) {
		absRootPath = filepath.Join(workingDir, rootPath)
	}

	result := make([]string, len(absPaths))
	for i, path := range absPaths {
		relPath, err := filepath.Rel(absRootPath, path)
		if err != nil {
			return nil, fmt.Errorf("file %s outside root path: %w", path, err)
		}

		if strings.HasPrefix(relPath, "..") {
			return nil, fmt.Errorf("file %s outside root path", path)
		}

		result[i] = "/" + relPath
	}

	return result, nil
}

// preparePathsForParser prepares paths by making them absolute and rebasing to root.
func preparePathsForParser(paths []string, rootPath string, workingDir string) ([]string, error) {
	if workingDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		workingDir = wd
	}

	absPaths, err := makePathsAbsolute(paths, workingDir)
	if err != nil {
		return nil, err
	}

	return rebasePathsToRoot(absPaths, rootPath, workingDir)
}

// getOSEnv returns the current OS environment as a map.
func getOSEnv() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

// determineFormat determines the format to use based on the provided format pointer and paths.
// If format is nil or points to an empty string, it infers from the paths.
// Returns an error if no format can be determined.
func determineFormat(formatName *string, paths ...*string) (*format.Format, error) {
	if formatName != nil && *formatName != "" {
		return format.Get(*formatName)
	}

	// Try to infer from paths
	for _, path := range paths {
		if path != nil && *path != "" {
			if name := utils.Ext(*path); name != "" {
				return format.Get(name)
			}
		}
	}

	return format.Get("")
}

// FormatOutput marshals the given data to the specified format.
// If format is nil or points to an empty string, it looks at the provided paths
// and uses the file extension of the first non-nil path as the format.
// Returns the marshaled bytes or an error if the format is unknown or marshaling fails.
func FormatOutput(data any, format *string, paths ...*string) ([]byte, error) {
	ft, err := determineFormat(format, paths...)
	if err != nil {
		return nil, err
	}

	return ft.MarshalStream([]any{data})
}

// Evaluate processes the specified files and returns the formatted output.
// If format is nil, it infers the format from the paths parameter (output path first, then input files).
// If env is nil, it uses the current OS environment.
func Evaluate(fx fs.FS, files []string, rootPath string, workingDir string, env map[string]string, format *string, paths ...*string) ([]byte, error) {
	if env == nil {
		env = getOSEnv()
	}

	evalFiles, err := preparePathsForParser(files, rootPath, workingDir)
	if err != nil {
		return nil, err
	}

	realFiles := make([]string, len(evalFiles))
	var inferredFormat string
	for i, path := range evalFiles {
		realPath, fileFormat, err := fileMatch(fx, path)
		if err != nil {
			return nil, fmt.Errorf("file %s: %w", path, err)
		}
		realFiles[i] = realPath

		if inferredFormat == "" {
			inferredFormat = fileFormat
		}
	}

	// Determine format to use - append inferredFormat to paths for fallback
	allPaths := append(paths, &inferredFormat)
	ft, err := determineFormat(format, allPaths...)
	if err != nil {
		return nil, err
	}

	return mergeFiles(fx, realFiles, ft, env)
}

// fileMatch attempts to find a file with the same base name as path, but
// possibly with a different supported extension. It is intended to support
// "virtual" filenames that auto-convert from the format of the underlying
// real file.
//
// Returns the real filename and the requested output format, or
// ("", "", error).
func fileMatch(fx fs.FS, path string) (string, string, error) {
	formatName := utils.Ext(path)
	if _, err := format.Get(formatName); err != nil {
		return "", "", err
	}

	withoutExt := strings.TrimSuffix(path, "."+formatName)

	if filepath.Base(withoutExt) == "-" {
		return path, formatName, nil
	}

	fileSystem := fsys.New(fx)
	realPath := fileSystem.FindFile(withoutExt)

	if realPath == "" {
		return "", "", fmt.Errorf("%s.*: %w", withoutExt, errors.ErrMissingFile)
	}

	return realPath, formatName, nil
}
