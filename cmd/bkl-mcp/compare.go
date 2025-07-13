package main

import (
	"context"
	"strings"

	"github.com/gopatchy/bkl"
)

type compareArgs struct {
	File1       string            `json:"file1"`
	File2       string            `json:"file2"`
	Format      string            `json:"format,omitempty"`
	FileSystem  map[string]string `json:"fileSystem,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Sort        string            `json:"sort,omitempty"`
}

type compareResponse struct {
	File1       string            `json:"file1"`
	File2       string            `json:"file2"`
	Diff        string            `json:"diff"`
	Operation   string            `json:"operation"`
	Environment map[string]string `json:"environment,omitempty"`
	Sort        string            `json:"sort,omitempty"`
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

	var sortPaths []string
	if args.Sort != "" {
		sortPaths = strings.Split(args.Sort, ",")
	}

	result, err := bkl.Compare(fsys, args.File1, args.File2, "/", workingDir, args.Environment, &args.Format, sortPaths)
	if err != nil {
		return nil, err
	}

	return &compareResponse{
		File1:       result.File1,
		File2:       result.File2,
		Diff:        result.Diff,
		Operation:   "compare",
		Environment: result.Environment,
		Sort:        strings.Join(result.Sort, ","),
	}, nil
}
