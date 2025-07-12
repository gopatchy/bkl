package main

import (
	"context"

	"github.com/gopatchy/bkl"
)

type compareArgs struct {
	File1       string            `json:"file1"`
	File2       string            `json:"file2"`
	Format      string            `json:"format,omitempty"`
	FileSystem  map[string]string `json:"fileSystem,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	SortPath    string            `json:"sortPath,omitempty"`
}

type compareResponse struct {
	File1       string            `json:"file1"`
	File2       string            `json:"file2"`
	Diff        string            `json:"diff"`
	Operation   string            `json:"operation"`
	Environment map[string]string `json:"environment,omitempty"`
	SortPath    string            `json:"sortPath,omitempty"`
}

func (s *Server) compareHandler(ctx context.Context, args compareArgs) (*compareResponse, error) {
	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	result, err := bkl.Compare(fsys, args.File1, args.File2, "/", workingDir, args.Environment, &args.Format, args.SortPath)
	if err != nil {
		return nil, err
	}

	return &compareResponse{
		File1:       result.File1,
		File2:       result.File2,
		Diff:        result.Diff,
		Operation:   "compare",
		Environment: result.Environment,
		SortPath:    result.SortPath,
	}, nil
}
