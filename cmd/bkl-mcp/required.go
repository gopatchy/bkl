package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gopatchy/bkl"
)

type requiredArgs struct {
	File       string            `json:"file"`
	Format     string            `json:"format,omitempty"`
	FileSystem map[string]string `json:"fileSystem,omitempty"`
	OutputPath string            `json:"outputPath,omitempty"`
}

type requiredResponse struct {
	File       string `json:"file"`
	Output     string `json:"output"`
	Operation  string `json:"operation"`
	OutputPath string `json:"outputPath,omitempty"`
}

func (s *Server) requiredHandler(ctx context.Context, args requiredArgs) (*requiredResponse, error) {
	workingDir := ""
	if args.FileSystem != nil {
		workingDir = "/"
	}

	fsys, err := getFileSystem(args.FileSystem)
	if err != nil {
		return nil, err
	}

	output, err := bkl.Required(fsys, args.File, "/", workingDir, &args.Format, &args.File)
	if err != nil {
		return nil, fmt.Errorf("required operation failed: %v", err)
	}

	if args.OutputPath != "" {
		if err := os.WriteFile(args.OutputPath, output, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write output to %s: %v", args.OutputPath, err)
		}
	}

	return &requiredResponse{
		File:       args.File,
		Output:     string(output),
		Operation:  "required",
		OutputPath: args.OutputPath,
	}, nil
}
