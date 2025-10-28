package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/gopatchy/bkl"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	tests    map[string]*bkl.DocExample
	sections []bkl.DocSection
}

func NewServer() (*Server, error) {
	tests, err := bkl.GetTests()
	if err != nil {
		return nil, fmt.Errorf("failed to load tests: %v", err)
	}

	sections, err := bkl.GetDocSections()
	if err != nil {
		return nil, fmt.Errorf("failed to load documentation sections: %v", err)
	}

	return &Server{
		tests:    tests,
		sections: sections,
	}, nil
}

type HandlerFunc[TArgs any, TResponse any] func(ctx context.Context, args TArgs) (*TResponse, error)

func wrapHandler[TArgs any, TResponse any](handler HandlerFunc[TArgs, TResponse]) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args TArgs
		if err := request.BindArguments(&args); err != nil {
			errorJSON, _ := json.Marshal(errorResponse{Error: err.Error()})
			return mcp.NewToolResultText(string(errorJSON)), nil
		}

		response, err := handler(ctx, args)
		if err != nil {
			errorJSON, _ := json.Marshal(errorResponse{Error: err.Error()})
			return mcp.NewToolResultText(string(errorJSON)), nil
		}

		resultJSON, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			errorJSON, _ := json.Marshal(errorResponse{Error: err.Error()})
			return mcp.NewToolResultText(string(errorJSON)), nil
		}

		return mcp.NewToolResultText(string(resultJSON)), nil
	}
}

func main() {
	srv, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	mcpServer := server.NewMCPServer(
		"bkl-mcp",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	formatParam := mcp.WithString("format",
		mcp.Description("Output format (yaml, json, toml) - will auto-detect if not specified"),
	)
	fileSystemParam := mcp.WithObject("fileSystem",
		mcp.Description("Map of filename to file content. If not provided, uses actual filesystem in current directory"),
	)

	queryTool := mcp.NewTool("query",
		mcp.WithDescription("Query bkl documentation and test examples by keywords"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("keywords",
			mcp.Required(),
			mcp.Description("Keywords to search for (comma-separated) in documentation sections and test examples"),
		),
	)
	mcpServer.AddTool(queryTool, wrapHandler(srv.queryHandler))

	getTool := mcp.NewTool("get",
		mcp.WithDescription("Get full content of a documentation section or test"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Type of content: 'documentation' or 'test'"),
		),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("ID of documentation section or name of test"),
		),
		mcp.WithString("source",
			mcp.Description("Source file for documentation (e.g., 'index', 'k8s'). Only applies to type='documentation'"),
		),
	)
	mcpServer.AddTool(getTool, wrapHandler(srv.getHandler))

	evaluateTool := mcp.NewTool("evaluate",
		mcp.WithDescription("Evaluate bkl files with given environment and return results"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("files",
			mcp.Description("Comma-separated list of files to evaluate (relative paths). Leave empty when using directory parameter."),
		),
		mcp.WithString("directory",
			mcp.Description("Directory path to evaluate all files within (alternative to files parameter)"),
		),
		mcp.WithString("pattern",
			mcp.Description("File pattern to match when using directory mode (e.g. '*.yaml', '*.bkl')"),
		),
		mcp.WithBoolean("includeOutput",
			mcp.Description("Include evaluated output for successful files when in directory mode (default: true)"),
		),
		formatParam,
		mcp.WithObject("environment",
			mcp.Description("Environment variables as key-value pairs"),
		),
		fileSystemParam,
		mcp.WithString("outputPath",
			mcp.Description("Optional path to write the output to (in addition to returning it)"),
		),
		mcp.WithString("sort",
			mcp.Description("Sort output documents by path (e.g. 'name' or 'metadata.priority'), comma-separated for multiple"),
		),
	)
	mcpServer.AddTool(evaluateTool, wrapHandler(srv.evaluateHandler))

	diffTool := mcp.NewTool("diff",
		mcp.WithDescription("Generate the minimal intermediate layer needed to create the target output from the base layer"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("baseFile",
			mcp.Required(),
			mcp.Description("Base file path"),
		),
		mcp.WithString("targetFile",
			mcp.Required(),
			mcp.Description("Target file path"),
		),
		mcp.WithString("selectors",
			mcp.Description("Selector expressions to match documents (e.g. 'metadata.name,metadata.type'), comma-separated for multiple"),
		),
		formatParam,
		fileSystemParam,
		mcp.WithString("outputPath",
			mcp.Description("Optional path to write the output to (in addition to returning it)"),
		),
	)
	mcpServer.AddTool(diffTool, wrapHandler(srv.diffHandler))

	intersectTool := mcp.NewTool("intersect",
		mcp.WithDescription("Generate the maximal base layer that the specified targets have in common"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("files",
			mcp.Required(),
			mcp.Description("Comma-separated list of files to intersect (requires at least 2 files)"),
		),
		mcp.WithString("selectors",
			mcp.Description("Selector expressions to match documents (e.g. 'metadata.name,metadata.type'), comma-separated for multiple"),
		),
		formatParam,
		fileSystemParam,
		mcp.WithString("outputPath",
			mcp.Description("Optional path to write the output to (in addition to returning it)"),
		),
	)
	mcpServer.AddTool(intersectTool, wrapHandler(srv.intersectHandler))

	requiredTool := mcp.NewTool("required",
		mcp.WithDescription("Generate a document containing just the required fields and their ancestors from the lower layer"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("File path to extract required fields from"),
		),
		formatParam,
		fileSystemParam,
		mcp.WithString("outputPath",
			mcp.Description("Optional path to write the output to (in addition to returning it)"),
		),
	)
	mcpServer.AddTool(requiredTool, wrapHandler(srv.requiredHandler))

	versionTool := mcp.NewTool("version",
		mcp.WithDescription("Get version and build information for bkl"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
	)
	mcpServer.AddTool(versionTool, wrapHandler(srv.versionHandler))

	issuePromptTool := mcp.NewTool("issue_prompt",
		mcp.WithDescription("Get guidance for filing an issue with minimal reproduction case"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
	)
	mcpServer.AddTool(issuePromptTool, wrapHandler(srv.issuePromptHandler))

	compareTool := mcp.NewTool("compare",
		mcp.WithDescription("Evaluate two bkl files and show text differences between their outputs"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("file1",
			mcp.Required(),
			mcp.Description("First file path to evaluate"),
		),
		mcp.WithString("file2",
			mcp.Required(),
			mcp.Description("Second file path to evaluate"),
		),
		formatParam,
		fileSystemParam,
		mcp.WithObject("environment",
			mcp.Description("Environment variables as key-value pairs"),
		),
		mcp.WithString("sort",
			mcp.Description("Sort output documents by path (e.g. 'name' or 'metadata.priority'), comma-separated for multiple"),
		),
	)
	mcpServer.AddTool(compareTool, wrapHandler(srv.compareHandler))

	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
