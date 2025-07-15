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

func runEvaluateTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, evaluate *bkl.DocEvaluate) {
	fileSystem := make(map[string]any)
	var evalFiles []string

	for _, input := range evaluate.Inputs {
		fileSystem[input.Filename] = input.Code
	}
	if len(evaluate.Inputs) > 0 {
		lastInput := evaluate.Inputs[len(evaluate.Inputs)-1]
		evalFiles = append(evalFiles, lastInput.Filename)
	}

	args := map[string]any{
		"files":      strings.Join(evalFiles, ","),
		"fileSystem": fileSystem,
	}

	format := getFormat(evaluate.Result.Languages)
	if format != nil {
		args["format"] = *format
	}

	if len(evaluate.Env) > 0 {
		args["environment"] = evaluate.Env
	}

	if len(evaluate.Sort) > 0 {
		args["sort"] = strings.Join(evaluate.Sort, ",")
	}

	result, err := client.CallTool(ctx, "evaluate", args)
	if err != nil {
		validateError(t, err, evaluate.Errors)
		return
	}

	output, err := extractOutput(result)
	validateError(t, err, evaluate.Errors)
	if err == nil {
		validateOutput(t, output, evaluate.Result.Code, 0)
	}
}

func runRequiredTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, required *bkl.DocRequired) {
	fileSystem := make(map[string]any)
	var evalFiles []string

	for _, input := range required.Inputs {
		fileSystem[input.Filename] = input.Code
		evalFiles = append(evalFiles, input.Filename)
	}

	if len(evalFiles) != 1 {
		t.Fatalf("Required tests require exactly 1 eval file, got %d", len(evalFiles))
	}

	args := map[string]any{
		"file":       evalFiles[0],
		"fileSystem": fileSystem,
	}

	format := getFormat(required.Result.Languages)
	if format != nil {
		args["format"] = *format
	}

	result, err := client.CallTool(ctx, "required", args)
	if err != nil {
		validateError(t, err, required.Errors)
		return
	}

	output, err := extractOutput(result)
	validateError(t, err, required.Errors)
	if err == nil {
		validateOutput(t, output, required.Result.Code, 0)
	}
}

func runIntersectTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, intersect *bkl.DocIntersect) {
	fileSystem := make(map[string]any)
	var evalFiles []string

	for _, input := range intersect.Inputs {
		fileSystem[input.Filename] = input.Code
		evalFiles = append(evalFiles, input.Filename)
	}

	if len(evalFiles) < 2 {
		t.Fatalf("Intersect tests require at least 2 eval files, got %d", len(evalFiles))
	}

	args := map[string]any{
		"files":      strings.Join(evalFiles, ","),
		"fileSystem": fileSystem,
	}

	format := getFormat(intersect.Result.Languages)
	if format != nil {
		args["format"] = *format
	}

	if len(intersect.Selector) > 0 {
		args["selectors"] = strings.Join(intersect.Selector, ",")
	}

	result, err := client.CallTool(ctx, "intersect", args)
	if err != nil {
		validateError(t, err, intersect.Errors)
		return
	}

	output, err := extractOutput(result)
	validateError(t, err, intersect.Errors)
	if err == nil {
		validateOutput(t, output, intersect.Result.Code, 0)
	}
}

func runDiffTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, diff *bkl.DocDiff) {
	fileSystem := make(map[string]any)
	fileSystem[diff.Base.Filename] = diff.Base.Code
	fileSystem[diff.Target.Filename] = diff.Target.Code

	args := map[string]any{
		"baseFile":   diff.Base.Filename,
		"targetFile": diff.Target.Filename,
		"fileSystem": fileSystem,
	}

	format := getFormat(diff.Result.Languages)
	if format != nil {
		args["format"] = *format
	}

	if len(diff.Selector) > 0 {
		args["selectors"] = strings.Join(diff.Selector, ",")
	}

	result, err := client.CallTool(ctx, "diff", args)
	if err != nil {
		validateError(t, err, diff.Errors)
		return
	}

	output, err := extractOutput(result)
	validateError(t, err, diff.Errors)
	if err == nil {
		validateOutput(t, output, diff.Result.Code, 0)
	}
}

func runCompareTestMCP(ctx context.Context, client *mcp.Client, t *testing.T, compare *bkl.DocCompare) {
	fileSystem := make(map[string]any)
	fileSystem[compare.Left.Filename] = compare.Left.Code
	fileSystem[compare.Right.Filename] = compare.Right.Code

	args := map[string]any{
		"file1":      compare.Left.Filename,
		"file2":      compare.Right.Filename,
		"fileSystem": fileSystem,
	}

	format := getFormat(compare.Result.Languages)
	if format != nil {
		args["format"] = *format
	}

	if len(compare.Env) > 0 {
		args["environment"] = compare.Env
	}

	if len(compare.Sort) > 0 {
		args["sort"] = strings.Join(compare.Sort, ",")
	}

	result, err := client.CallTool(ctx, "compare", args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	output, err := extractDiff(result)
	if err != nil {
		t.Fatalf("Failed to extract diff: %v", err)
	}

	validateOutput(t, output, compare.Result.Code, 2)
}

func extractOutput(result *mcp.ToolResponse) ([]byte, error) {
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

			if output, ok := response["output"].(string); ok {
				return []byte(output), nil
			}

			return nil, fmt.Errorf("no output field in response")
		}
	}

	return nil, fmt.Errorf("no text content in tool response")
}

func extractDiff(result *mcp.ToolResponse) ([]byte, error) {
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

			if diff, ok := response["diff"].(string); ok {
				return []byte(diff), nil
			}

			return nil, fmt.Errorf("no diff field in response")
		}
	}

	return nil, fmt.Errorf("no text content in tool response")
}

func TestMCP(t *testing.T) {
	t.Parallel()

	tests, err := bkl.GetAllTests()
	if err != nil {
		t.Fatalf("Failed to get all tests: %v", err)
	}

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
	defer cmd.Process.Kill()

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
	defer cancel()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize MCP client: %v", err)
	}

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
