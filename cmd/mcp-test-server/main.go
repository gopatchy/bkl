package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pelletier/go-toml/v2"
)

type TestCase struct {
	Description string            `toml:"description"`
	Eval        []string          `toml:"eval"`
	Format      string            `toml:"format"`
	Expected    string            `toml:"expected,omitempty"`
	Error       string            `toml:"error,omitempty"`
	Files       map[string]string `toml:"files"`
	Diff        bool              `toml:"diff,omitempty"`
	Intersect   bool              `toml:"intersect,omitempty"`
	Required    bool              `toml:"required,omitempty"`
	Skip        bool              `toml:"skip,omitempty"`
}

var tests map[string]*TestCase

func loadTests() error {
	data, err := os.ReadFile("tests.toml")
	if err != nil {
		return err
	}

	if err := toml.Unmarshal(data, &tests); err != nil {
		return err
	}
	return nil
}

func main() {
	// Load tests on startup
	if err := loadTests(); err != nil {
		log.Fatalf("Failed to load tests: %v", err)
	}

	// Create a new MCP server
	mcpServer := server.NewMCPServer(
		"bkl-test-server",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Register tools
	getTestTool := mcp.NewTool("get_test",
		mcp.WithDescription("Get a specific test case by name"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the test to retrieve"),
		),
	)
	mcpServer.AddTool(getTestTool, getTestHandler)

	listTestsTool := mcp.NewTool("list_tests",
		mcp.WithDescription("List all test names with optional filtering"),
		mcp.WithString("filter",
			mcp.Description("Filter tests by name substring"),
		),
	)
	mcpServer.AddTool(listTestsTool, listTestsHandler)

	compareTestsTool := mcp.NewTool("compare_tests",
		mcp.WithDescription("Compare two test cases to identify differences"),
		mcp.WithString("test1",
			mcp.Required(),
			mcp.Description("Name of first test"),
		),
		mcp.WithString("test2",
			mcp.Required(),
			mcp.Description("Name of second test"),
		),
	)
	mcpServer.AddTool(compareTestsTool, compareTestsHandler)

	findSimilarTestsTool := mcp.NewTool("find_similar_tests",
		mcp.WithDescription("Find tests with similar names or descriptions"),
		mcp.WithString("pattern",
			mcp.Required(),
			mcp.Description("Pattern to search for in test names and descriptions"),
		),
	)
	mcpServer.AddTool(findSimilarTestsTool, findSimilarTestsHandler)

	analyzeRedundancyTool := mcp.NewTool("analyze_redundancy",
		mcp.WithDescription("Analyze potential redundancy between tests based on coverage overlap data"),
		mcp.WithString("coverage_data",
			mcp.Required(),
			mcp.Description("Coverage overlap data from coverage-analyzer tool"),
		),
	)
	mcpServer.AddTool(analyzeRedundancyTool, analyzeRedundancyHandler)

	// Start the stdio transport
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// Tool handlers

func getTestHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	test, exists := tests[name]
	if !exists {
		return mcp.NewToolResultText(fmt.Sprintf("Test '%s' not found", name)), nil
	}

	testJSON, err := json.MarshalIndent(test, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(testJSON)), nil
}

func listTestsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filter := request.GetString("filter", "")

	var testNames []string
	for name := range tests {
		if filter == "" || strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
			testNames = append(testNames, name)
		}
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d tests:\n%s", len(testNames), strings.Join(testNames, "\n"))), nil
}

func compareTestsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	test1Name, err := request.RequireString("test1")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	test2Name, err := request.RequireString("test2")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	test1, exists1 := tests[test1Name]
	test2, exists2 := tests[test2Name]

	if !exists1 || !exists2 {
		return mcp.NewToolResultText("One or both tests not found"), nil
	}

	comparison := compareTests(test1, test2, test1Name, test2Name)
	return mcp.NewToolResultText(comparison), nil
}

func findSimilarTestsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pattern, err := request.RequireString("pattern")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	similar := findSimilarTests(pattern)
	return mcp.NewToolResultText(similar), nil
}

func analyzeRedundancyHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	coverageData, err := request.RequireString("coverage_data")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	analysis := analyzeRedundancy(coverageData)
	return mcp.NewToolResultText(analysis), nil
}

// Helper functions (unchanged from original)

func compareTests(test1, test2 *TestCase, name1, name2 string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Comparing %s vs %s:\n\n", name1, name2))

	// Compare descriptions
	if test1.Description != test2.Description {
		sb.WriteString(fmt.Sprintf("Descriptions differ:\n  %s: %s\n  %s: %s\n\n", name1, test1.Description, name2, test2.Description))
	} else {
		sb.WriteString(fmt.Sprintf("Same description: %s\n\n", test1.Description))
	}

	// Compare eval files
	if len(test1.Eval) != len(test2.Eval) || !slicesEqual(test1.Eval, test2.Eval) {
		sb.WriteString(fmt.Sprintf("Eval files differ:\n  %s: %v\n  %s: %v\n\n", name1, test1.Eval, name2, test2.Eval))
	}

	// Compare expected output
	if test1.Expected != test2.Expected {
		sb.WriteString("Expected outputs differ\n\n")
	}

	// Compare error expectations
	if test1.Error != test2.Error {
		sb.WriteString(fmt.Sprintf("Error expectations differ:\n  %s: %s\n  %s: %s\n\n", name1, test1.Error, name2, test2.Error))
	}

	// Compare file counts
	if len(test1.Files) != len(test2.Files) {
		sb.WriteString(fmt.Sprintf("Different number of files:\n  %s: %d files\n  %s: %d files\n\n",
			name1, len(test1.Files), name2, len(test2.Files)))
	}

	// Check if files have same content
	filesIdentical := true
	for filename, content1 := range test1.Files {
		content2, exists := test2.Files[filename]
		if !exists || content1 != content2 {
			filesIdentical = false
			break
		}
	}

	if filesIdentical && len(test1.Files) == len(test2.Files) {
		sb.WriteString("File contents are identical\n")
	} else {
		sb.WriteString("File contents differ\n")
	}

	return sb.String()
}

func findSimilarTests(pattern string) string {
	pattern = strings.ToLower(pattern)
	var results []string

	for name, test := range tests {
		nameLower := strings.ToLower(name)
		descLower := strings.ToLower(test.Description)

		if strings.Contains(nameLower, pattern) || strings.Contains(descLower, pattern) {
			results = append(results, fmt.Sprintf("%s: %s", name, test.Description))
		}
	}

	return fmt.Sprintf("Found %d tests matching '%s':\n%s", len(results), pattern, strings.Join(results, "\n"))
}

func analyzeRedundancy(coverageData string) string {
	var sb strings.Builder
	sb.WriteString("Analyzing test redundancy based on coverage overlap:\n\n")

	// Parse the coverage data to find 100% overlaps
	lines := strings.Split(coverageData, "\n")
	var redundantPairs []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "→ 100% overlap with") {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				test1 := parts[0]
				test2 := parts[4]

				// Get both tests
				t1, ok1 := tests[test1]
				t2, ok2 := tests[test2]

				if ok1 && ok2 {
					// Check if they're truly redundant
					if areTestsRedundant(t1, t2) {
						redundantPairs = append(redundantPairs, fmt.Sprintf("%s and %s appear redundant", test1, test2))
					}
				}
			}
		}
	}

	if len(redundantPairs) > 0 {
		sb.WriteString("Potentially redundant test pairs:\n")
		for _, pair := range redundantPairs {
			sb.WriteString("  - " + pair + "\n")
		}
	} else {
		sb.WriteString("No clearly redundant test pairs found based on 100% coverage overlap.\n")
		sb.WriteString("Most tests with identical coverage are testing different edge cases or features.\n")
	}

	return sb.String()
}

func areTestsRedundant(test1, test2 *TestCase) bool {
	// Tests are redundant if they have:
	// 1. Same description (or very similar)
	// 2. Same expected output
	// 3. Same error expectation
	// 4. Very similar file structures

	if test1.Description == test2.Description {
		return true
	}

	if test1.Expected == test2.Expected && test1.Error == test2.Error && len(test1.Files) == len(test2.Files) {
		// Check if file contents are essentially the same
		filesSimilar := true
		for filename, content1 := range test1.Files {
			if content2, exists := test2.Files[filename]; exists {
				if content1 != content2 {
					filesSimilar = false
					break
				}
			}
		}
		return filesSimilar
	}

	return false
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
