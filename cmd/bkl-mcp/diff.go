package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gopatchy/bkl"
)

type diffArgs struct {
	BaseFile   string            `json:"baseFile"`
	TargetFile string            `json:"targetFile"`
	Selector   string            `json:"selector,omitempty"`
	Format     string            `json:"format,omitempty"`
	FileSystem map[string]string `json:"fileSystem,omitempty"`
	OutputPath string            `json:"outputPath,omitempty"`
}

type diffResponse struct {
	BaseFile   string `json:"baseFile"`
	TargetFile string `json:"targetFile"`
	Output     string `json:"output"`
	Operation  string `json:"operation"`
	OutputPath string `json:"outputPath,omitempty"`
}

func (s *Server) diffHandler(ctx context.Context, args diffArgs) (*diffResponse, error) {
	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	output, err := bkl.Diff(fsys, args.BaseFile, args.TargetFile, "/", workingDir, args.Selector, &args.Format, &args.BaseFile, &args.TargetFile)
	if err != nil {
		return nil, fmt.Errorf("diff operation failed: %v", err)
	}

	if args.OutputPath != "" {
		if err := os.WriteFile(args.OutputPath, output, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write output to %s: %v", args.OutputPath, err)
		}
	}

	return &diffResponse{
		BaseFile:   args.BaseFile,
		TargetFile: args.TargetFile,
		Output:     string(output),
		Operation:  "diff",
		OutputPath: args.OutputPath,
	}, nil
}
