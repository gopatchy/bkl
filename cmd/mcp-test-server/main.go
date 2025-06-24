package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

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

type TestResult struct {
	Name           string
	UniqueLines    int
	UniqueCoverage map[string]bool
	ActualCoverage map[string]bool
}

type OverlapResult struct {
	TestName       string  `json:"test_name"`
	OverlapsWith   string  `json:"overlaps_with"`
	SharedLines    int     `json:"shared_lines"`
	TotalLines     int     `json:"total_lines"`
	OverlapPercent float64 `json:"overlap_percent"`
}

type CoverageAnalyzer struct {
	baselineCoverage map[string]bool
	testExcluded     map[string]map[string]bool
	testActual       map[string]map[string]bool
	mu               sync.Mutex
	counter          int64
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

	analyzeCoverageTool := mcp.NewTool("analyze_coverage",
		mcp.WithDescription("Analyze test coverage contributions - pass JSON object with optional 'tests' array"),
	)
	mcpServer.AddTool(analyzeCoverageTool, analyzeCoverageHandler)

	findZeroCoverageTool := mcp.NewTool("find_zero_coverage_tests",
		mcp.WithDescription("Find tests with zero unique coverage contribution"),
	)
	mcpServer.AddTool(findZeroCoverageTool, findZeroCoverageHandler)

	getCoverageSummaryTool := mcp.NewTool("get_coverage_summary",
		mcp.WithDescription("Get coverage summary statistics"),
	)
	mcpServer.AddTool(getCoverageSummaryTool, getCoverageSummaryHandler)

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

func analyzeCoverageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get test names array if provided
	testNames := request.GetStringSlice("tests", nil)

	result, err := analyzeCoverage(testNames)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

func findZeroCoverageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result, err := findZeroCoverageTests()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

func getCoverageSummaryHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result, err := getCoverageSummary()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
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

// Coverage analysis functions

func extractTestNames() []string {
	var names []string
	for name := range tests {
		if !strings.HasSuffix(name, ".files") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func filterTestNames(allTests []string, filter []string) []string {
	if len(filter) == 0 {
		return allTests
	}
	filterMap := make(map[string]bool)
	for _, f := range filter {
		filterMap[f] = true
	}
	var filtered []string
	for _, name := range allTests {
		if filterMap[name] {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

func runCoverageAndExtractLines(excludeTest, includeTest string, counter *int64) (map[string]bool, error) {
	id := atomic.AddInt64(counter, 1)
	coverFile := fmt.Sprintf("cover_%d.out", id)

	args := []string{"test", "-run", "TestLanguage", fmt.Sprintf("-coverprofile=%s", coverFile)}
	if excludeTest != "" {
		args = append(args, fmt.Sprintf("-test.exclude=%s", excludeTest))
	}
	if includeTest != "" {
		args = append(args, fmt.Sprintf("-test.filter=%s", includeTest))
	}

	cmd := exec.Command("go", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(coverFile)
		return nil, fmt.Errorf("go test failed: %v\nstderr: %s", err, stderr.String())
	}

	covered, err := parseCoverageFile(coverFile)
	os.Remove(coverFile)
	return covered, err
}

func parseCoverageFile(filename string) (map[string]bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	covered := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		if fields[len(fields)-1] == "1" {
			covered[fields[0]] = true
		}
	}

	return covered, scanner.Err()
}

func analyzeCoverage(testFilter []string) (interface{}, error) {
	allTestNames := extractTestNames()

	// If specific tests are requested, only analyze those
	var testNamesToAnalyze []string
	if len(testFilter) > 0 {
		testNamesToAnalyze = filterTestNames(allTestNames, testFilter)
		if len(testNamesToAnalyze) == 0 {
			return map[string]interface{}{
				"baseline_lines": 0,
				"results":        []map[string]interface{}{},
				"overlaps":       []OverlapResult{},
				"error":          "No matching tests found",
			}, nil
		}
	} else {
		testNamesToAnalyze = allTestNames
	}

	analyzer := &CoverageAnalyzer{
		testExcluded: make(map[string]map[string]bool),
		testActual:   make(map[string]map[string]bool),
	}

	// Use fixed concurrency value of 8
	concurrency := 8

	// Run baseline coverage with ALL tests (needed for accurate unique line calculation)
	baselineCoverage, err := runCoverageAndExtractLines("", "", &analyzer.counter)
	if err != nil {
		return nil, fmt.Errorf("failed to get baseline coverage: %v", err)
	}
	analyzer.baselineCoverage = baselineCoverage

	// Analyze unique coverage only for requested tests
	results := analyzer.analyzeUniqueCoverage(testNamesToAnalyze, concurrency)

	// Analyze actual coverage for zero-unique tests
	analyzer.analyzeActualCoverage(results, testNamesToAnalyze, concurrency)

	// Find overlaps
	overlaps := analyzer.findOverlaps(results)

	// Sort results
	sort.Slice(results, func(i, j int) bool {
		if results[i].UniqueLines == results[j].UniqueLines {
			return results[i].Name < results[j].Name
		}
		return results[i].UniqueLines < results[j].UniqueLines
	})

	// Create compact results without the coverage maps
	compactResults := make([]map[string]interface{}, len(results))
	for i, r := range results {
		compactResults[i] = map[string]interface{}{
			"name":         r.Name,
			"unique_lines": r.UniqueLines,
			"actual_lines": len(r.ActualCoverage),
		}
	}

	// Filter overlaps to only include requested tests
	var filteredOverlaps []OverlapResult
	if len(testFilter) > 0 {
		testSet := make(map[string]bool)
		for _, t := range testFilter {
			testSet[t] = true
		}
		for _, o := range overlaps {
			if testSet[o.TestName] {
				filteredOverlaps = append(filteredOverlaps, o)
			}
		}
	} else {
		filteredOverlaps = overlaps
	}

	return map[string]interface{}{
		"baseline_lines": len(baselineCoverage),
		"results":        compactResults,
		"overlaps":       filteredOverlaps,
	}, nil
}

func (ca *CoverageAnalyzer) analyzeUniqueCoverage(testNames []string, concurrencyLimit int) []TestResult {
	results := make([]TestResult, len(testNames))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrencyLimit)

	for i, testName := range testNames {
		wg.Add(1)
		go func(index int, name string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			coverageWithout, err := runCoverageAndExtractLines(name, "", &ca.counter)
			if err != nil {
				results[index] = TestResult{Name: name, UniqueLines: 0}
				return
			}

			ca.mu.Lock()
			ca.testExcluded[name] = coverageWithout
			ca.mu.Unlock()

			uniqueLines := 0
			testUniqueCoverage := make(map[string]bool)

			for line := range ca.baselineCoverage {
				if !coverageWithout[line] {
					uniqueLines++
					testUniqueCoverage[line] = true
				}
			}

			results[index] = TestResult{
				Name:           name,
				UniqueLines:    uniqueLines,
				UniqueCoverage: testUniqueCoverage,
			}
		}(i, testName)
	}

	wg.Wait()
	return results
}

func (ca *CoverageAnalyzer) analyzeActualCoverage(results []TestResult, testNames []string, concurrencyLimit int) {
	var zeroUniqueTests []int
	for i, r := range results {
		if r.UniqueLines == 0 {
			zeroUniqueTests = append(zeroUniqueTests, i)
		}
	}

	if len(zeroUniqueTests) == 0 {
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrencyLimit)

	for _, idx := range zeroUniqueTests {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			testName := results[index].Name
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			coverage, err := runCoverageAndExtractLines("", testName, &ca.counter)
			if err != nil {
				return
			}

			ca.mu.Lock()
			ca.testActual[testName] = coverage
			results[index].ActualCoverage = coverage
			ca.mu.Unlock()
		}(idx)
	}

	wg.Wait()
}

func (ca *CoverageAnalyzer) findOverlaps(results []TestResult) []OverlapResult {
	var overlaps []OverlapResult

	for _, r := range results {
		if r.UniqueLines > 0 || len(r.ActualCoverage) == 0 {
			continue
		}

		maxShared := 0
		bestMatch := ""
		bestMatchTotal := 0

		for _, other := range results {
			if other.Name == r.Name {
				continue
			}

			otherCoverage := other.ActualCoverage
			if len(otherCoverage) == 0 && other.UniqueLines > 0 {
				otherCoverage = make(map[string]bool)
				for line := range other.UniqueCoverage {
					otherCoverage[line] = true
				}
				if excluded, ok := ca.testExcluded[other.Name]; ok {
					for line := range ca.baselineCoverage {
						if !excluded[line] {
							otherCoverage[line] = true
						}
					}
				}
			}

			if len(otherCoverage) == 0 {
				continue
			}

			shared := 0
			for line := range r.ActualCoverage {
				if otherCoverage[line] {
					shared++
				}
			}

			if shared > maxShared || (shared == maxShared && len(otherCoverage) < bestMatchTotal) {
				maxShared = shared
				bestMatch = other.Name
				bestMatchTotal = len(otherCoverage)
			}
		}

		if bestMatch != "" && maxShared > 0 {
			overlaps = append(overlaps, OverlapResult{
				TestName:       r.Name,
				OverlapsWith:   bestMatch,
				SharedLines:    maxShared,
				TotalLines:     len(r.ActualCoverage),
				OverlapPercent: float64(maxShared) / float64(len(r.ActualCoverage)) * 100,
			})
		}
	}

	sort.Slice(overlaps, func(i, j int) bool {
		if overlaps[i].OverlapPercent == overlaps[j].OverlapPercent {
			return overlaps[i].SharedLines > overlaps[j].SharedLines
		}
		return overlaps[i].OverlapPercent > overlaps[j].OverlapPercent
	})

	return overlaps
}

func findZeroCoverageTests() (interface{}, error) {
	testNames := extractTestNames()

	analyzer := &CoverageAnalyzer{
		testExcluded: make(map[string]map[string]bool),
		testActual:   make(map[string]map[string]bool),
	}

	baselineCoverage, err := runCoverageAndExtractLines("", "", &analyzer.counter)
	if err != nil {
		return nil, fmt.Errorf("failed to get baseline coverage: %v", err)
	}
	analyzer.baselineCoverage = baselineCoverage

	results := analyzer.analyzeUniqueCoverage(testNames, 8)

	var zeroTests []string
	for _, r := range results {
		if r.UniqueLines == 0 {
			zeroTests = append(zeroTests, r.Name)
		}
	}

	sort.Strings(zeroTests)
	return map[string]interface{}{
		"total_tests":         len(testNames),
		"zero_coverage_tests": zeroTests,
		"count":               len(zeroTests),
	}, nil
}

func getCoverageSummary() (interface{}, error) {
	testNames := extractTestNames()

	analyzer := &CoverageAnalyzer{
		testExcluded: make(map[string]map[string]bool),
		testActual:   make(map[string]map[string]bool),
	}

	baselineCoverage, err := runCoverageAndExtractLines("", "", &analyzer.counter)
	if err != nil {
		return nil, fmt.Errorf("failed to get baseline coverage: %v", err)
	}
	analyzer.baselineCoverage = baselineCoverage

	results := analyzer.analyzeUniqueCoverage(testNames, 8)

	distribution := make(map[int]int)
	zeroCount := 0
	totalUniqueLines := 0

	for _, r := range results {
		distribution[r.UniqueLines]++
		totalUniqueLines += r.UniqueLines
		if r.UniqueLines == 0 {
			zeroCount++
		}
	}

	return map[string]interface{}{
		"total_tests":           len(testNames),
		"baseline_lines":        len(baselineCoverage),
		"zero_coverage_tests":   zeroCount,
		"total_unique_lines":    totalUniqueLines,
		"coverage_distribution": distribution,
	}, nil
}
