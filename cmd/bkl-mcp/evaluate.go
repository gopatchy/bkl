package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gopatchy/bkl"
)

type evaluateArgs struct {
	Files         string            `json:"files,omitempty"`
	Directory     string            `json:"directory,omitempty"`
	Pattern       string            `json:"pattern,omitempty"`
	IncludeOutput *bool             `json:"includeOutput,omitempty"`
	Format        string            `json:"format,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	FileSystem    map[string]string `json:"fileSystem,omitempty"`
	OutputPath    string            `json:"outputPath,omitempty"`
	SortPath      string            `json:"sortPath,omitempty"`
}

type evaluateResponse struct {
	Files        []string          `json:"files,omitempty"`
	Directory    string            `json:"directory,omitempty"`
	Pattern      string            `json:"pattern,omitempty"`
	TotalFiles   int               `json:"totalFiles,omitempty"`
	SuccessCount int               `json:"successCount,omitempty"`
	ErrorCount   int               `json:"errorCount,omitempty"`
	Results      []evaluateResult  `json:"results,omitempty"`
	Output       string            `json:"output"`
	Operation    string            `json:"operation"`
	Environment  map[string]string `json:"environment,omitempty"`
	OutputPath   string            `json:"outputPath,omitempty"`
}

type evaluateResult struct {
	Path   string `json:"path"`
	Error  string `json:"error,omitempty"`
	Output string `json:"output,omitempty"`
}

func (s *Server) evaluateHandler(ctx context.Context, args evaluateArgs) (*evaluateResponse, error) {
	if args.Directory != "" && args.Files != "" {
		return nil, fmt.Errorf("cannot specify both files and directory parameters")
	}

	if args.Directory == "" && args.Files == "" {
		return nil, fmt.Errorf("must specify either files or directory parameter")
	}

	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	if args.Directory != "" {
		includeOutput := true
		if args.IncludeOutput != nil {
			includeOutput = *args.IncludeOutput
		}

		results, err := bkl.EvaluateTree(fsys, args.Directory, args.Pattern, args.Environment, &args.Format)
		if err != nil {
			return nil, fmt.Errorf("directory evaluation failed: %v", err)
		}

		successCount, errorCount := 0, 0
		for _, result := range results {
			if result.Error == nil {
				successCount++
			} else {
				errorCount++
			}
		}

		finalResults := []evaluateResult{}
		for _, result := range results {
			r := evaluateResult{
				Path: result.Path,
			}
			if result.Error != nil {
				r.Error = result.Error.Error()
			}
			if includeOutput && result.Output != "" {
				r.Output = result.Output
			}
			finalResults = append(finalResults, r)
		}

		return &evaluateResponse{
			Directory:    args.Directory,
			Pattern:      args.Pattern,
			TotalFiles:   len(results),
			SuccessCount: successCount,
			ErrorCount:   errorCount,
			Results:      finalResults,
			Operation:    "evaluate_tree",
			Environment:  args.Environment,
		}, nil
	}

	fileFields := strings.Split(args.Files, ",")
	files := []string{}
	for _, f := range fileFields {
		trimmed := strings.TrimSpace(f)
		if trimmed != "" {
			files = append(files, trimmed)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files provided")
	}

	paths := []*string{}
	for _, file := range files {
		paths = append(paths, &file)
	}

	output, err := bkl.Evaluate(fsys, files, "/", workingDir, args.Environment, &args.Format, args.SortPath, paths...)
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %v", err)
	}

	if args.OutputPath != "" {
		if err := os.WriteFile(args.OutputPath, output, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write output to %s: %v", args.OutputPath, err)
		}
	}

	return &evaluateResponse{
		Files:       files,
		Output:      string(output),
		Operation:   "evaluate",
		Environment: args.Environment,
		OutputPath:  args.OutputPath,
	}, nil
}
