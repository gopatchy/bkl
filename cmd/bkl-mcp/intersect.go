package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gopatchy/bkl"
)

type intersectArgs struct {
	Files      string            `json:"files"`
	Selectors  string            `json:"selectors,omitempty"`
	Format     string            `json:"format,omitempty"`
	FileSystem map[string]string `json:"fileSystem,omitempty"`
	OutputPath string            `json:"outputPath,omitempty"`
}

type intersectResponse struct {
	Files      []string `json:"files"`
	Output     string   `json:"output"`
	Operation  string   `json:"operation"`
	OutputPath string   `json:"outputPath,omitempty"`
}

func (s *Server) intersectHandler(ctx context.Context, args intersectArgs) (*intersectResponse, error) {
	files := strings.Split(args.Files, ",")
	paths := []*string{}
	for _, f := range files {
		paths = append(paths, &f)
	}

	if len(files) < 2 {
		return nil, fmt.Errorf("intersect operation requires at least 2 files")
	}
	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	var selectors []string
	if args.Selectors != "" {
		selectors = strings.Split(args.Selectors, ",")
	}

	output, err := bkl.Intersect(fsys, files, "/", workingDir, selectors, &args.Format, paths...)
	if err != nil {
		return nil, fmt.Errorf("intersect operation failed: %v", err)
	}

	if args.OutputPath != "" {
		if err := os.WriteFile(args.OutputPath, output, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write output to %s: %v", args.OutputPath, err)
		}
	}

	return &intersectResponse{
		Files:      files,
		Output:     string(output),
		Operation:  "intersect",
		OutputPath: args.OutputPath,
	}, nil
}
