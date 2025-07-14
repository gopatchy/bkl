package bkl_test

import (
	"bytes"
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

func TestMCP(t *testing.T) {
	tests, err := bkl.GetTests()
	if err != nil {
		t.Fatalf("Failed to get tests: %v", err)
	}

	filterTests := map[string]bool{}
	if *testFilter != "" {
		for _, name := range strings.Split(*testFilter, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				if _, ok := tests[name]; !ok {
					t.Fatalf("Test %q not found", name)
				}
				filterTests[name] = true
			}
		}
	}

	excludeTests := map[string]bool{}
	if *testExclude != "" {
		for _, name := range strings.Split(*testExclude, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				if _, ok := tests[name]; !ok {
					t.Fatalf("Test %q not found", name)
				}
				excludeTests[name] = true
			}
		}
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
		if len(filterTests) > 0 && !filterTests[testName] {
			continue
		}

		if excludeTests[testName] {
			continue
		}

		if testCase.Benchmark {
			continue
		}

		// Skip tests with root path (not supported via MCP yet)
		if (testCase.Evaluate != nil && testCase.Evaluate.Root != "") ||
			(testCase.Required != nil && testCase.Required.Root != "") {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			output, err := runTestCaseViaMCP(ctx, client, testCase, testName, t)

			// Get expected errors from operation-specific structure
			var expectedErrors []string
			switch {
			case testCase.Evaluate != nil:
				expectedErrors = testCase.Evaluate.Errors
			case testCase.Required != nil:
				expectedErrors = testCase.Required.Errors
			case testCase.Intersect != nil:
				expectedErrors = testCase.Intersect.Errors
			case testCase.Diff != nil:
				expectedErrors = testCase.Diff.Errors
				// Note: Compare doesn't have Errors field
			}

			if len(expectedErrors) > 0 {
				if err == nil {
					t.Fatalf("Expected error containing one of %v, but got no error", expectedErrors)
				}

				errorFound := false
				for _, expectedError := range expectedErrors {
					if strings.Contains(err.Error(), expectedError) {
						errorFound = true
						break
					}
				}

				if !errorFound {
					t.Fatalf("Expected error containing one of %v, but got: %v", expectedErrors, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Get expected result from operation-specific structure
			var expected string
			switch {
			case testCase.Required != nil:
				expected = testCase.Required.Result.Code
			case testCase.Intersect != nil:
				expected = testCase.Intersect.Result.Code
			case testCase.Diff != nil:
				expected = testCase.Diff.Result.Code
			case testCase.Compare != nil:
				expected = testCase.Compare.Result.Code
			case testCase.Evaluate != nil:
				expected = testCase.Evaluate.Result.Code
			}

			if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(expected))) {
				t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expected, output)
			}
		})
	}
}

func runTestCaseViaMCP(ctx context.Context, client *mcp.Client, testCase *bkl.TestCase, testName string, t *testing.T) ([]byte, error) {
	// Build filesystem from operation-specific structure
	fileSystem := make(map[string]any)
	var format *string
	var evalFiles []string

	switch {
	case testCase.Required != nil:
		for _, input := range testCase.Required.Inputs {
			fileSystem[input.Filename] = input.Code
			evalFiles = append(evalFiles, input.Filename)
		}
		format = getFormat(testCase.Required.Result.Languages)

	case testCase.Intersect != nil:
		for _, input := range testCase.Intersect.Inputs {
			fileSystem[input.Filename] = input.Code
			evalFiles = append(evalFiles, input.Filename)
		}
		format = getFormat(testCase.Intersect.Result.Languages)

	case testCase.Diff != nil:
		fileSystem[testCase.Diff.Base.Filename] = testCase.Diff.Base.Code
		fileSystem[testCase.Diff.Target.Filename] = testCase.Diff.Target.Code
		evalFiles = []string{testCase.Diff.Base.Filename, testCase.Diff.Target.Filename}
		format = getFormat(testCase.Diff.Result.Languages)

	case testCase.Compare != nil:
		fileSystem[testCase.Compare.Left.Filename] = testCase.Compare.Left.Code
		fileSystem[testCase.Compare.Right.Filename] = testCase.Compare.Right.Code
		evalFiles = []string{testCase.Compare.Left.Filename, testCase.Compare.Right.Filename}
		format = getFormat(testCase.Compare.Result.Languages)

	case testCase.Evaluate != nil:
		for _, input := range testCase.Evaluate.Inputs {
			fileSystem[input.Filename] = input.Code
		}
		// Only evaluate the last file to match TestBKL behavior
		if len(testCase.Evaluate.Inputs) > 0 {
			lastInput := testCase.Evaluate.Inputs[len(testCase.Evaluate.Inputs)-1]
			evalFiles = append(evalFiles, lastInput.Filename)
		}
		format = getFormat(testCase.Evaluate.Result.Languages)
	}

	// Don't set a default format - let the server infer it from file extension
	// if format == nil {
	//	defaultFormat := "yaml"
	//	format = &defaultFormat
	// }

	var result *mcp.ToolResponse
	var err error

	switch {
	case testCase.Required != nil:
		if len(evalFiles) != 1 {
			return nil, fmt.Errorf("Required tests require exactly 1 eval file, got %d", len(evalFiles))
		}

		args := map[string]any{
			"file":       evalFiles[0],
			"fileSystem": fileSystem,
		}
		if format != nil {
			args["format"] = *format
		}
		result, err = client.CallTool(ctx, "required", args)

	case testCase.Intersect != nil:
		if len(evalFiles) < 2 {
			return nil, fmt.Errorf("Intersect tests require at least 2 eval files, got %d", len(evalFiles))
		}

		args := map[string]any{
			"files":      strings.Join(evalFiles, ","),
			"fileSystem": fileSystem,
		}
		if format != nil {
			args["format"] = *format
		}
		if len(testCase.Intersect.Selector) > 0 {
			args["selectors"] = strings.Join(testCase.Intersect.Selector, ",")
		}
		result, err = client.CallTool(ctx, "intersect", args)

	case testCase.Diff != nil:
		if len(evalFiles) != 2 {
			return nil, fmt.Errorf("Diff tests require exactly 2 eval files, got %d", len(evalFiles))
		}

		args := map[string]any{
			"baseFile":   evalFiles[0],
			"targetFile": evalFiles[1],
			"fileSystem": fileSystem,
		}
		if format != nil {
			args["format"] = *format
		}
		if len(testCase.Diff.Selector) > 0 {
			args["selectors"] = strings.Join(testCase.Diff.Selector, ",")
		}
		result, err = client.CallTool(ctx, "diff", args)

	case testCase.Compare != nil:
		if len(evalFiles) != 2 {
			return nil, fmt.Errorf("Compare tests require exactly 2 eval files, got %d", len(evalFiles))
		}

		args := map[string]any{
			"file1":      evalFiles[0],
			"file2":      evalFiles[1],
			"fileSystem": fileSystem,
		}
		if format != nil {
			args["format"] = *format
		}
		if len(testCase.Compare.Env) > 0 {
			args["environment"] = testCase.Compare.Env
		}
		if len(testCase.Compare.Sort) > 0 {
			args["sort"] = strings.Join(testCase.Compare.Sort, ",")
		}
		result, err = client.CallTool(ctx, "compare", args)

	default:
		// Default to evaluate
		args := map[string]any{
			"files":      strings.Join(evalFiles, ","),
			"fileSystem": fileSystem,
		}
		if format != nil {
			args["format"] = *format
		}

		if testCase.Evaluate != nil && len(testCase.Evaluate.Env) > 0 {
			args["environment"] = testCase.Evaluate.Env
		}

		if testCase.Evaluate != nil && len(testCase.Evaluate.Sort) > 0 {
			args["sort"] = strings.Join(testCase.Evaluate.Sort, ",")
		}

		result, err = client.CallTool(ctx, "evaluate", args)
	}

	if err != nil {
		return nil, err
	}

	for _, content := range result.Content {
		if content.Type == "text" && content.TextContent != nil {
			text := content.TextContent.Text

			var response map[string]any
			if err := json.Unmarshal([]byte(text), &response); err != nil {
				return nil, fmt.Errorf("failed to parse JSON response: %v", err)
			}

			// Check for error field first
			if errMsg, ok := response["error"].(string); ok {
				return nil, fmt.Errorf("%s", errMsg)
			}

			if output, ok := response["output"].(string); ok {
				return []byte(output), nil
			}

			if diff, ok := response["diff"].(string); ok {
				return []byte(diff), nil
			}

			return nil, fmt.Errorf("no output or diff field in response")
		}
	}

	return nil, fmt.Errorf("no text content in tool response")
}
