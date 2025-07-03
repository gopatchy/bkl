package bkl_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

func TestMCPFormatAutoDetection(t *testing.T) {
	// Start MCP server
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

	// Log stderr
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize MCP client: %v", err)
	}

	tests := []struct {
		name           string
		tool           string
		args           map[string]any
		expectedFormat string
		expectedOutput string
	}{
		{
			name: "Auto-detect JSON from outputPath",
			tool: "evaluate",
			args: map[string]any{
				"files":      "a.yaml",
				"outputPath": "output.json",
				"fileSystem": map[string]any{
					"a.yaml": "a: 1\nb: 2\n",
				},
			},
			expectedFormat: "json",
			expectedOutput: `{"a":1,"b":2}`,
		},
		{
			name: "Auto-detect TOML from outputPath",
			tool: "evaluate",
			args: map[string]any{
				"files":      "a.yaml",
				"outputPath": "output.toml",
				"fileSystem": map[string]any{
					"a.yaml": "a: 1\nb: 2\n",
				},
			},
			expectedFormat: "toml",
			expectedOutput: "a = 1\nb = 2",
		},
		{
			name: "Auto-detect from input file extension",
			tool: "evaluate",
			args: map[string]any{
				"files": "a.json",
				"fileSystem": map[string]any{
					"a.json": `{"a":1,"b":2}`,
				},
			},
			expectedFormat: "json",
			expectedOutput: `{"a":1,"b":2}`,
		},
		{
			name: "Auto-detect YAML from outputPath in diff",
			tool: "diff",
			args: map[string]any{
				"baseFile":   "base.yaml",
				"targetFile": "target.yaml",
				"outputPath": "diff.yaml",
				"fileSystem": map[string]any{
					"base.yaml":   "a: 1\n",
					"target.yaml": "a: 1\nb: 2\n",
				},
			},
			expectedFormat: "yaml",
			expectedOutput: "$match: {}\nb: 2",
		},
		{
			name: "Auto-detect JSON from outputPath in intersect",
			tool: "intersect",
			args: map[string]any{
				"files":      "a.yaml,b.yaml",
				"outputPath": "common.json",
				"fileSystem": map[string]any{
					"a.yaml": "x: 1\ny: 2\n",
					"b.yaml": "x: 1\nz: 3\n",
				},
			},
			expectedFormat: "json",
			expectedOutput: `{"x":1}`,
		},
		{
			name: "Auto-detect JSON from outputPath in required",
			tool: "required",
			args: map[string]any{
				"file":       "a.yaml",
				"outputPath": "required.json",
				"fileSystem": map[string]any{
					"a.yaml": "a: $required\nb: 2\n",
				},
			},
			expectedFormat: "json",
			expectedOutput: `{"a":"$required"}`,
		},
		{
			name: "Explicit format overrides auto-detection",
			tool: "evaluate",
			args: map[string]any{
				"files":      "a.yaml",
				"format":     "toml",
				"outputPath": "output.json", // outputPath suggests JSON, but explicit format wins
				"fileSystem": map[string]any{
					"a.yaml": "a: 1\nb: 2\n",
				},
			},
			expectedFormat: "toml",
			expectedOutput: "a = 1\nb = 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := client.CallTool(ctx, tt.tool, tt.args)
			if err != nil {
				t.Fatalf("Failed to call tool %s: %v", tt.tool, err)
			}

			// Extract response from result
			var response map[string]any
			for _, content := range result.Content {
				if content.Type == "text" && content.TextContent != nil {
					text := content.TextContent.Text
					// Check for error responses
					if strings.Contains(text, "operation failed:") {
						t.Fatalf("Operation failed: %s", text)
					}
					if err := json.Unmarshal([]byte(text), &response); err != nil {
						t.Fatalf("Failed to parse response: %v\nResponse text: %s", err, text)
					}
					break
				}
			}

			// Check format
			if format, ok := response["format"].(string); ok {
				if format != tt.expectedFormat {
					t.Errorf("Expected format %q, got %q", tt.expectedFormat, format)
				}
			} else {
				t.Errorf("No format in response")
			}

			// Check output
			if output, ok := response["output"].(string); ok {
				// Normalize whitespace for comparison
				expected := strings.TrimSpace(tt.expectedOutput)
				actual := strings.TrimSpace(output)
				if actual != expected {
					t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expected, actual)
				}
			} else {
				t.Errorf("No output in response")
			}
		})
	}
}
