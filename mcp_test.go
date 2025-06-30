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

		if testCase.RootPath != "" {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			output, err := runTestCaseViaMCP(ctx, client, testCase)

			if len(testCase.Errors) > 0 {
				if err == nil {
					t.Fatalf("Expected error containing one of %v, but got no error", testCase.Errors)
				}

				errorFound := false
				for _, expectedError := range testCase.Errors {
					if strings.Contains(err.Error(), expectedError) {
						errorFound = true
						break
					}
				}

				if !errorFound {
					t.Fatalf("Expected error containing one of %v, but got: %v", testCase.Errors, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(testCase.Expected))) {
				t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", testCase.Expected, output)
			}
		})
	}
}

func runTestCaseViaMCP(ctx context.Context, client *mcp.Client, testCase *bkl.TestCase) ([]byte, error) {
	format := testCase.Format
	if format == "" {
		format = "yaml"
	}
	fileSystem := make(map[string]any)
	for filename, content := range testCase.Files {
		fileSystem[filename] = content
	}

	var result *mcp.ToolResponse
	var err error

	switch {
	case testCase.Required:
		if len(testCase.Eval) != 1 {
			return nil, fmt.Errorf("Required tests require exactly 1 eval file, got %d", len(testCase.Eval))
		}

		result, err = client.CallTool(ctx, "required", map[string]any{
			"file":       testCase.Eval[0],
			"format":     format,
			"fileSystem": fileSystem,
		})

	case testCase.Intersect:
		if len(testCase.Eval) < 2 {
			return nil, fmt.Errorf("Intersect tests require at least 2 eval files, got %d", len(testCase.Eval))
		}

		result, err = client.CallTool(ctx, "intersect", map[string]any{
			"files":      strings.Join(testCase.Eval, ","),
			"format":     format,
			"fileSystem": fileSystem,
		})

	case testCase.Diff:
		if len(testCase.Eval) != 2 {
			return nil, fmt.Errorf("Diff tests require exactly 2 eval files, got %d", len(testCase.Eval))
		}

		result, err = client.CallTool(ctx, "diff", map[string]any{
			"baseFile":   testCase.Eval[0],
			"targetFile": testCase.Eval[1],
			"format":     format,
			"fileSystem": fileSystem,
		})

	default:
		args := map[string]any{
			"files":      strings.Join(testCase.Eval, ","),
			"format":     format,
			"fileSystem": fileSystem,
		}

		if len(testCase.Env) > 0 {
			args["environment"] = testCase.Env
		}

		result, err = client.CallTool(ctx, "evaluate", args)
	}

	if err != nil {
		return nil, err
	}

	for _, content := range result.Content {
		if content.Type == "text" && content.TextContent != nil {
			text := content.TextContent.Text

			if strings.Contains(text, "Evaluation failed:") || strings.Contains(text, "operation failed:") {
				var errResponse map[string]any
				if err := json.Unmarshal([]byte(text), &errResponse); err == nil {
					return nil, fmt.Errorf("%s", text)
				}
				return nil, fmt.Errorf("%s", text)
			}

			var response map[string]any
			if err := json.Unmarshal([]byte(text), &response); err != nil {
				return nil, fmt.Errorf("failed to parse JSON response: %v", err)
			}

			if output, ok := response["output"].(string); ok {
				return []byte(output), nil
			}

			return nil, fmt.Errorf("no output field in response")
		}
	}

	return nil, fmt.Errorf("no text content in tool response")
}
