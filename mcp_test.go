package bkl_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gopatchy/bkl"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func buildFileSystem(inputs []*bkl.DocLayer) map[string]any {
	fileSystem := make(map[string]any)
	for _, input := range inputs {
		fileSystem[input.Filename] = input.Code
	}
	return fileSystem
}

func getFilenames(inputs []*bkl.DocLayer) []string {
	var files []string
	for _, input := range inputs {
		files = append(files, input.Filename)
	}
	return files
}

func buildBaseArgs(fileSystem map[string]any, format *string) map[string]any {
	args := map[string]any{
		"fileSystem": fileSystem,
	}
	if format != nil {
		args["format"] = *format
	}
	return args
}

func callToolAndValidate(ctx context.Context, client *mcp.Client, t *testing.T, tool string, args map[string]any, expectedErrors []string, expectedOutput string, removeLines int) {
	result, err := client.CallTool(ctx, tool, args)
	if err != nil {
		validateError(t, err, expectedErrors)
		return
	}

	output, err := extractOutput(result)
	validateResult(t, err, output, expectedErrors, expectedOutput, removeLines)
}

func callToolAndValidateDiff(ctx context.Context, client *mcp.Client, t *testing.T, tool string, args map[string]any, expectedOutput string) {
	result, err := client.CallTool(ctx, tool, args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	output, err := extractDiff(result)
	if err != nil {
		t.Fatalf("Failed to extract diff: %v", err)
	}

	validateOutput(t, output, expectedOutput, 2)
}

func runEvaluateTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, evaluate *bkl.DocEvaluate) {
	fileSystem := buildFileSystem(evaluate.Inputs)

	var evalFiles []string
	if len(evaluate.Inputs) > 0 {
		lastInput := evaluate.Inputs[len(evaluate.Inputs)-1]
		evalFiles = append(evalFiles, lastInput.Filename)
	}

	format := getFormat(evaluate.Result.Languages)
	args := buildBaseArgs(fileSystem, format)
	args["files"] = strings.Join(evalFiles, ",")

	if len(evaluate.Env) > 0 {
		args["environment"] = evaluate.Env
	}

	if len(evaluate.Sort) > 0 {
		args["sort"] = strings.Join(evaluate.Sort, ",")
	}

	callToolAndValidate(ctx, client, t, "evaluate", args, evaluate.Errors, evaluate.Result.Code, 0)
}

func runRequiredTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, required *bkl.DocRequired) {
	fileSystem := buildFileSystem(required.Inputs)
	evalFiles := getFilenames(required.Inputs)

	format := getFormat(required.Result.Languages)
	args := buildBaseArgs(fileSystem, format)
	args["file"] = evalFiles[0]

	callToolAndValidate(ctx, client, t, "required", args, required.Errors, required.Result.Code, 0)
}

func runIntersectTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, intersect *bkl.DocIntersect) {
	fileSystem := buildFileSystem(intersect.Inputs)
	evalFiles := getFilenames(intersect.Inputs)

	format := getFormat(intersect.Result.Languages)
	args := buildBaseArgs(fileSystem, format)
	args["files"] = strings.Join(evalFiles, ",")

	if len(intersect.Selector) > 0 {
		args["selectors"] = strings.Join(intersect.Selector, ",")
	}

	callToolAndValidate(ctx, client, t, "intersect", args, intersect.Errors, intersect.Result.Code, 0)
}

func runDiffTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, diff *bkl.DocDiff) {
	fileSystem := make(map[string]any)
	fileSystem[diff.Base.Filename] = diff.Base.Code
	fileSystem[diff.Target.Filename] = diff.Target.Code

	format := getFormat(diff.Result.Languages)
	args := buildBaseArgs(fileSystem, format)
	args["baseFile"] = diff.Base.Filename
	args["targetFile"] = diff.Target.Filename

	if len(diff.Selector) > 0 {
		args["selectors"] = strings.Join(diff.Selector, ",")
	}

	callToolAndValidate(ctx, client, t, "diff", args, diff.Errors, diff.Result.Code, 0)
}

func runCompareTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, compare *bkl.DocCompare) {
	fileSystem := make(map[string]any)
	fileSystem[compare.Left.Filename] = compare.Left.Code
	fileSystem[compare.Right.Filename] = compare.Right.Code

	format := getFormat(compare.Result.Languages)
	args := buildBaseArgs(fileSystem, format)
	args["file1"] = compare.Left.Filename
	args["file2"] = compare.Right.Filename

	if len(compare.Env) > 0 {
		args["environment"] = compare.Env
	}

	if len(compare.Sort) > 0 {
		args["sort"] = strings.Join(compare.Sort, ",")
	}

	callToolAndValidateDiff(ctx, client, t, "compare", args, compare.Result.Code)
}

func extractField(result *mcp.ToolResponse, fieldName string) ([]byte, error) {
	for _, content := range result.Content {
		if content.Type == "text" && content.TextContent != nil {
			text := content.TextContent.Text

			var response map[string]any
			if err := json.Unmarshal([]byte(text), &response); err != nil {
				return nil, fmt.Errorf("failed to parse JSON response: %v", err)
			}

			if errMsg, ok := response["error"].(string); ok {
				return nil, fmt.Errorf("%s", errMsg)
			}

			if value, ok := response[fieldName].(string); ok {
				return []byte(value), nil
			}

			return nil, fmt.Errorf("no %s field in response", fieldName)
		}
	}

	return nil, fmt.Errorf("no text content in tool response")
}

func extractOutput(result *mcp.ToolResponse) ([]byte, error) {
	return extractField(result, "output")
}

func extractDiff(result *mcp.ToolResponse) ([]byte, error) {
	return extractField(result, "diff")
}

func setupMCPServer(t *testing.T) (*exec.Cmd, *mcp.Client, context.Context, context.CancelFunc) {
	cmd := exec.Command("go", "run", "./cmd/bkl-mcp/")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to get stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				t.Logf("bkl-mcp stderr: %s", buf[:n])
			}
		}
	}()

	transport := stdio.NewStdioServerTransportWithIO(stdout, stdin)
	client := mcp.NewClient(transport)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize MCP client: %v", err)
	}

	return cmd, client, ctx, cancel
}

func TestMCP(t *testing.T) {
	t.Parallel()

	tests, err := bkl.GetAllTests()
	if err != nil {
		t.Fatalf("Failed to get all tests: %v", err)
	}

	cmd, client, ctx, cancel := setupMCPServer(t)
	defer cmd.Process.Kill()
	defer cancel()

	for testName, testCase := range tests {
		if testCase.Benchmark {
			continue
		}

		// Skip tests with root path (not supported via MCP yet)
		if (testCase.Evaluate != nil && testCase.Evaluate.Root != "") ||
			(testCase.Required != nil && testCase.Required.Root != "") {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			switch {
			case testCase.Evaluate != nil:
				runEvaluateTestMCP(ctx, client, t, testCase.Evaluate)
			case testCase.Required != nil:
				runRequiredTestMCP(ctx, client, t, testCase.Required)
			case testCase.Intersect != nil:
				runIntersectTestMCP(ctx, client, t, testCase.Intersect)
			case testCase.Diff != nil:
				runDiffTestMCP(ctx, client, t, testCase.Diff)
			case testCase.Compare != nil:
				runCompareTestMCP(ctx, client, t, testCase.Compare)
				// Convert and Fixit are not supported via MCP
			}
		})
	}
}
