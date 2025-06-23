package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pelletier/go-toml/v2"
)

type TestResult struct {
	Name        string
	UniqueLines int
}

type CoverageAnalyzer struct {
	baselineCoverage map[string]bool
	mu               sync.Mutex
	counter          int64
}

func main() {
	analyzer := &CoverageAnalyzer{}

	// Run baseline coverage
	fmt.Println("Running baseline coverage with all tests...")
	baselineCoverage, err := analyzer.runCoverageAndExtractLines("")
	if err != nil {
		log.Fatalf("Failed to get baseline coverage: %v", err)
	}
	analyzer.baselineCoverage = baselineCoverage
	fmt.Printf("Baseline: %d lines covered\n", len(baselineCoverage))

	// Extract test names
	fmt.Println("Extracting test names...")
	testNames, err := extractTestNames("tests.toml")
	if err != nil {
		log.Fatalf("Failed to extract test names: %v", err)
	}
	fmt.Printf("Found %d tests\n", len(testNames))

	// Analyze tests in parallel
	fmt.Printf("Analyzing %d tests in parallel...\n", len(testNames))
	results := analyzer.analyzeTestsInParallel(testNames)

	// Print results
	printResults(results)
}

func (ca *CoverageAnalyzer) runCoverageAndExtractLines(excludeTest string) (map[string]bool, error) {
	// Generate unique filename
	id := atomic.AddInt64(&ca.counter, 1)
	coverFile := fmt.Sprintf("cover_%d.out", id)

	// Build command
	args := []string{"test", "-run", "TestLanguage", fmt.Sprintf("-coverprofile=%s", coverFile)}
	if excludeTest != "" {
		args = append(args, fmt.Sprintf("-test.exclude=%s", excludeTest))
	}

	// Run go test
	cmd := exec.Command("go", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Clean up on error
		os.Remove(coverFile)
		return nil, fmt.Errorf("go test failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse coverage file
	covered, err := parseCoverageFile(coverFile)

	// Always clean up
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

		// Check if line is covered (last field is 1)
		if fields[len(fields)-1] == "1" {
			covered[fields[0]] = true
		}
	}

	return covered, scanner.Err()
}

func extractTestNames(filename string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var tests map[string]interface{}
	if err := toml.Unmarshal(data, &tests); err != nil {
		return nil, err
	}

	var names []string
	for name, value := range tests {
		// Skip if it's not a map (i.e., not a test)
		if _, ok := value.(map[string]interface{}); !ok {
			continue
		}
		// Skip .files tests
		if strings.HasSuffix(name, ".files") {
			continue
		}
		names = append(names, name)
	}

	sort.Strings(names)
	return names, nil
}

func (ca *CoverageAnalyzer) analyzeTestsInParallel(testNames []string) []TestResult {
	results := make([]TestResult, len(testNames))
	var wg sync.WaitGroup
	var progressCounter int64

	// Use a semaphore to limit concurrency
	semaphore := make(chan struct{}, 8)

	for i, testName := range testNames {
		wg.Add(1)
		go func(index int, name string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Show progress
			count := atomic.AddInt64(&progressCounter, 1)
			if count%10 == 0 {
				fmt.Printf("\rProgress: %d/%d tests processed...", count, len(testNames))
			}

			// Run coverage without this test
			coverageWithout, err := ca.runCoverageAndExtractLines(name)
			if err != nil {
				log.Printf("Error analyzing test %s: %v", name, err)
				results[index] = TestResult{Name: name, UniqueLines: 0}
				return
			}

			// Count unique lines
			uniqueLines := 0
			for line := range ca.baselineCoverage {
				if !coverageWithout[line] {
					uniqueLines++
				}
			}

			results[index] = TestResult{Name: name, UniqueLines: uniqueLines}
		}(i, testName)
	}

	wg.Wait()
	fmt.Printf("\rProgress: %d/%d tests processed...Done!\n", len(testNames), len(testNames))

	return results
}

func printResults(results []TestResult) {
	// Sort by unique lines
	sort.Slice(results, func(i, j int) bool {
		if results[i].UniqueLines == results[j].UniqueLines {
			return results[i].Name < results[j].Name
		}
		return results[i].UniqueLines < results[j].UniqueLines
	})

	// Count zero coverage tests
	zeroCount := 0
	for _, r := range results {
		if r.UniqueLines == 0 {
			zeroCount++
		}
	}

	fmt.Println("\n=========================================")
	fmt.Println("SUMMARY")
	fmt.Println("=========================================")
	fmt.Printf("Total tests analyzed: %d\n", len(results))
	fmt.Printf("Tests contributing zero coverage: %d\n", zeroCount)
	fmt.Println()

	// Show zero coverage tests
	if zeroCount > 0 {
		fmt.Println("Tests with ZERO coverage contribution:")
		zeroCoverageTests := []string{}
		for _, r := range results {
			if r.UniqueLines == 0 {
				zeroCoverageTests = append(zeroCoverageTests, r.Name)
			}
		}

		// Print in columns
		printInColumns(zeroCoverageTests, 80)
		fmt.Println()
	}

	// Show top contributors
	fmt.Println("Top 20 coverage contributors (by unique lines covered):")
	topStart := len(results) - 20
	if topStart < 0 {
		topStart = 0
	}
	for i := len(results) - 1; i >= topStart && i >= 0; i-- {
		if results[i].UniqueLines > 0 {
			fmt.Printf("  %-50s %4d lines\n", results[i].Name, results[i].UniqueLines)
		}
	}

	// Show bottom non-zero contributors
	fmt.Println("\nBottom 10 non-zero contributors:")
	bottomCount := 0
	for _, r := range results {
		if r.UniqueLines > 0 {
			fmt.Printf("  %-50s %4d lines\n", r.Name, r.UniqueLines)
			bottomCount++
			if bottomCount >= 10 {
				break
			}
		}
	}

	// Show distribution
	fmt.Println("\nCoverage contribution distribution:")
	distribution := make(map[int]int)
	for _, r := range results {
		distribution[r.UniqueLines]++
	}

	var lines []int
	for line := range distribution {
		lines = append(lines, line)
	}
	sort.Ints(lines)

	for _, line := range lines {
		fmt.Printf("  %4d tests contribute %d lines\n", distribution[line], line)
	}

	fmt.Println("\nNote: Tests with 0 coverage contribution may still be valuable for:")
	fmt.Println("  - Ensuring error cases are handled correctly")
	fmt.Println("  - Testing output formatting")
	fmt.Println("  - Validating edge cases")
	fmt.Println("  - Preventing regressions")
}

func printInColumns(items []string, width int) {
	// Sort items first
	sort.Strings(items)

	// Calculate column width based on longest item
	maxLen := 0
	for _, item := range items {
		if len(item) > maxLen {
			maxLen = len(item)
		}
	}

	colWidth := maxLen + 2
	numCols := width / colWidth
	if numCols < 1 {
		numCols = 1
	}

	for i, item := range items {
		fmt.Printf("%-*s", colWidth, item)
		if (i+1)%numCols == 0 {
			fmt.Println()
		}
	}
	if len(items)%numCols != 0 {
		fmt.Println()
	}
}
