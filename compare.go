package bkl

import (
	"fmt"
	"io/fs"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

type CompareResult struct {
	File1       string
	File2       string
	Format      string
	Diff        string
	Environment map[string]string
	SortPath    string
}

func Compare(fsys fs.FS, file1, file2 string, rootPath, workingDir string, env map[string]string, format *string, sortPath string) (*CompareResult, error) {
	output1, err := Evaluate(fsys, []string{file1}, rootPath, workingDir, env, format, sortPath, &file1)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate %s: %w", file1, err)
	}

	output2, err := Evaluate(fsys, []string{file2}, rootPath, workingDir, env, format, sortPath, &file2)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate %s: %w", file2, err)
	}

	edits := myers.ComputeEdits(span.URIFromPath(file1), string(output1), string(output2))
	unified := fmt.Sprint(gotextdiff.ToUnified(file1, file2, string(output1), edits))

	finalFormat := ""
	if format != nil {
		finalFormat = *format
	}

	result := &CompareResult{
		File1:       file1,
		File2:       file2,
		Format:      finalFormat,
		Diff:        unified,
		Environment: env,
		SortPath:    sortPath,
	}

	return result, nil
}
