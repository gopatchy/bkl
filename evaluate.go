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
	"strings"

	"github.com/gopatchy/bkl/internal/file"
	"github.com/gopatchy/bkl/internal/merge"
	"github.com/gopatchy/bkl/internal/utils"
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
//   - $defer (marks document for deferred processing)
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
// After all phases, documents marked with $defer are applied to the output.
//
// # Document Layer Matching Logic
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

// Evaluate processes the specified files and returns the formatted output.
// If format is nil, it infers the format from the paths parameter (output path first, then input files).
// If env is nil, it uses the current OS environment.
func Evaluate(fx fs.FS, files []string, rootPath string, workingDir string, env map[string]string, format *string, sortPath string, paths ...*string) ([]byte, error) {
	if env == nil {
		env = getOSEnv()
	}

	evalFiles, err := utils.PreparePathsForParser(files, rootPath, workingDir)
	if err != nil {
		return nil, err
	}

	realFiles := make([]string, len(evalFiles))
	var inferredFormat string
	for i, path := range evalFiles {
		realPath, fileFormat, err := file.FileMatch(fx, path)
		if err != nil {
			return nil, fmt.Errorf("file %s: %w", path, err)
		}
		realFiles[i] = realPath

		if inferredFormat == "" {
			inferredFormat = fileFormat
		}
	}

	allPaths := append(paths, &inferredFormat)
	ft, err := determineFormat(format, allPaths...)
	if err != nil {
		return nil, err
	}

	return merge.Files(fx, realFiles, ft, env, sortPath)
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
