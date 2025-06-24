package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pelletier/go-toml/v2"
)

type TestResult struct {
	Name           string
	UniqueLines    int
	UniqueCoverage map[string]bool // Lines covered by this test alone
	ActualCoverage map[string]bool // All lines covered by this test
}

type OverlapResult struct {
	TestName       string
	OverlapsWith   string
	SharedLines    int
	TotalLines     int
	OverlapPercent float64
}

type CoverageAnalyzer struct {
	baselineCoverage map[string]bool
	testExcluded     map[string]map[string]bool // test name -> coverage when test is excluded
	testActual       map[string]map[string]bool // test name -> coverage when only this test runs
	mu               sync.Mutex
	counter          int64
}

const usage = `coverage-analyzer - Analyze test coverage contributions

Usage:
  coverage-analyzer [flags]

Examples:
  # Analyze all tests
  coverage-analyzer

  # Analyze specific tests
  coverage-analyzer -tests=testFoo,testBar

  # Show only zero-coverage tests
  coverage-analyzer -zero-only

  # Show grouped output
  coverage-analyzer -grouped

  # Export to JSON
  coverage-analyzer -format=json > coverage-report.json

  # Analyze tests with low coverage contribution
  coverage-analyzer -threshold=5

Flags:
`

var (
	testsFlag     = flag.String("tests", "", "Comma-separated list of specific tests to analyze")
	zeroOnlyFlag  = flag.Bool("zero-only", false, "Show only tests with zero coverage")
	groupedFlag   = flag.Bool("grouped", false, "Show tests grouped by coverage ranges")
	formatFlag    = flag.String("format", "text", "Output format: text, json")
	excludeFlag   = flag.String("exclude", "", "Comma-separated list of tests to exclude")
	verboseFlag   = flag.Bool("verbose", false, "Show detailed progress")
	thresholdFlag = flag.Int("threshold", -1, "Only show tests at or below this coverage threshold")
	concurrency   = flag.Int("concurrency", runtime.NumCPU(), "Number of parallel test runs")
)

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}
	flag.Parse()

	analyzer := &CoverageAnalyzer{
		testExcluded: make(map[string]map[string]bool),
		testActual:   make(map[string]map[string]bool),
	}

	// Run baseline coverage
	if *verboseFlag {
		fmt.Println("Running baseline coverage with all tests...")
	}
	baselineCoverage, err := analyzer.runCoverageAndExtractLines("", "")
	if err != nil {
		log.Fatalf("Failed to get baseline coverage: %v", err)
	}
	analyzer.baselineCoverage = baselineCoverage
	fmt.Printf("Baseline: %d lines covered\n", len(baselineCoverage))

	// Extract test names
	if *verboseFlag {
		fmt.Println("Extracting test names...")
	}
	testNames, err := extractTestNames("tests.toml")
	if err != nil {
		log.Fatalf("Failed to extract test names: %v", err)
	}
	
	// Apply filters
	testNames = filterTests(testNames, *testsFlag, *excludeFlag)
	
	fmt.Printf("Found %d tests to analyze\n", len(testNames))

	// Phase 1: Analyze unique coverage by excluding each test
	if *verboseFlag {
		fmt.Printf("\nPhase 1: Analyzing unique coverage for %d tests...\n", len(testNames))
	}
	results := analyzer.analyzeUniqueCoverage(testNames, *concurrency)

	// Phase 2: For zero-unique tests, get their actual coverage
	if *verboseFlag {
		fmt.Printf("\nPhase 2: Analyzing actual coverage for zero-unique tests...\n")
	}
	analyzer.analyzeActualCoverage(results, testNames, *concurrency)

	// Find overlaps for zero-coverage tests
	if *verboseFlag {
		fmt.Println("\nAnalyzing overlaps...")
	}
	overlaps := analyzer.findOverlaps(results)

	// Apply threshold filter if specified
	if *thresholdFlag >= 0 {
		var filtered []TestResult
		for _, r := range results {
			if r.UniqueLines <= *thresholdFlag {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Apply zero-only filter if specified
	if *zeroOnlyFlag {
		var filtered []TestResult
		for _, r := range results {
			if r.UniqueLines == 0 {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Print results based on format
	switch *formatFlag {
	case "json":
		printJSONResults(results, overlaps)
	case "text":
		if *groupedFlag {
			printGroupedResults(results)
		} else {
			printResults(results, overlaps)
		}
	default:
		log.Fatalf("Unknown format: %s", *formatFlag)
	}
}

func (ca *CoverageAnalyzer) runCoverageAndExtractLines(excludeTest, includeTest string) (map[string]bool, error) {
	// Generate unique filename
	id := atomic.AddInt64(&ca.counter, 1)
	coverFile := fmt.Sprintf("cover_%d.out", id)

	// Build command
	args := []string{"test", "-run", "TestLanguage", fmt.Sprintf("-coverprofile=%s", coverFile)}
	if excludeTest != "" {
		args = append(args, fmt.Sprintf("-test.exclude=%s", excludeTest))
	}
	if includeTest != "" {
		args = append(args, fmt.Sprintf("-test.filter=%s", includeTest))
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

func filterTests(testNames []string, includeFilter, excludeFilter string) []string {
	// Apply include filter if specified
	if includeFilter != "" {
		includes := strings.Split(includeFilter, ",")
		includeMap := make(map[string]bool)
		for _, inc := range includes {
			includeMap[strings.TrimSpace(inc)] = true
		}
		
		var filtered []string
		for _, name := range testNames {
			if includeMap[name] {
				filtered = append(filtered, name)
			}
		}
		testNames = filtered
	}
	
	// Apply exclude filter if specified
	if excludeFilter != "" {
		excludes := strings.Split(excludeFilter, ",")
		excludeMap := make(map[string]bool)
		for _, exc := range excludes {
			excludeMap[strings.TrimSpace(exc)] = true
		}
		
		var filtered []string
		for _, name := range testNames {
			if !excludeMap[name] {
				filtered = append(filtered, name)
			}
		}
		testNames = filtered
	}
	
	return testNames
}

func (ca *CoverageAnalyzer) analyzeUniqueCoverage(testNames []string, concurrencyLimit int) []TestResult {
	results := make([]TestResult, len(testNames))
	var wg sync.WaitGroup
	var progressCounter int64

	// Use a semaphore to limit concurrency
	semaphore := make(chan struct{}, concurrencyLimit)

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
			coverageWithout, err := ca.runCoverageAndExtractLines(name, "")
			if err != nil {
				log.Printf("Error analyzing test %s: %v", name, err)
				results[index] = TestResult{Name: name, UniqueLines: 0}
				return
			}

			// Store coverage data when this test is excluded
			ca.mu.Lock()
			ca.testExcluded[name] = coverageWithout
			ca.mu.Unlock()

			// Count unique lines (lines only covered by this test)
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
	fmt.Printf("\rProgress: %d/%d tests processed...Done!\n", len(testNames), len(testNames))

	return results
}

func (ca *CoverageAnalyzer) analyzeActualCoverage(results []TestResult, testNames []string, concurrencyLimit int) {
	// Count zero-unique tests
	var zeroUniqueTests []int
	for i, r := range results {
		if r.UniqueLines == 0 {
			zeroUniqueTests = append(zeroUniqueTests, i)
		}
	}

	if len(zeroUniqueTests) == 0 {
		return
	}

	fmt.Printf("Found %d tests with zero unique coverage, analyzing their actual coverage...\n", len(zeroUniqueTests))

	var wg sync.WaitGroup
	var progressCounter int64
	semaphore := make(chan struct{}, concurrencyLimit)

	for _, idx := range zeroUniqueTests {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			testName := results[index].Name

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Show progress
			count := atomic.AddInt64(&progressCounter, 1)
			if count%10 == 0 {
				fmt.Printf("\rProgress: %d/%d zero-unique tests processed...", count, len(zeroUniqueTests))
			}

			// Run coverage with only this test
			coverage, err := ca.runCoverageAndExtractLines("", testName)
			if err != nil {
				log.Printf("Error getting actual coverage for test %s: %v", testName, err)
				return
			}

			// Store actual coverage
			ca.mu.Lock()
			ca.testActual[testName] = coverage
			results[index].ActualCoverage = coverage
			ca.mu.Unlock()
		}(idx)
	}

	wg.Wait()
	fmt.Printf("\rProgress: %d/%d zero-unique tests processed...Done!\n", len(zeroUniqueTests), len(zeroUniqueTests))
}

func (ca *CoverageAnalyzer) findOverlaps(results []TestResult) []OverlapResult {
	var overlaps []OverlapResult

	// For each zero-unique test with actual coverage, find best overlap
	for _, r := range results {
		if r.UniqueLines > 0 || len(r.ActualCoverage) == 0 {
			continue
		}

		// Find which other test has the most overlap
		maxShared := 0
		bestMatch := ""
		bestMatchTotal := 0

		for _, other := range results {
			if other.Name == r.Name {
				continue
			}

			// Get other test's actual coverage
			otherCoverage := other.ActualCoverage
			if len(otherCoverage) == 0 && other.UniqueLines > 0 {
				// For tests with unique coverage, reconstruct their coverage
				otherCoverage = make(map[string]bool)
				// Add unique lines
				for line := range other.UniqueCoverage {
					otherCoverage[line] = true
				}
				// Add shared lines (baseline - excluded)
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

			// Count shared lines
			shared := 0
			for line := range r.ActualCoverage {
				if otherCoverage[line] {
					shared++
				}
			}

			// Use absolute shared count as main criteria
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

	// Sort by overlap percentage
	sort.Slice(overlaps, func(i, j int) bool {
		if overlaps[i].OverlapPercent == overlaps[j].OverlapPercent {
			return overlaps[i].SharedLines > overlaps[j].SharedLines
		}
		return overlaps[i].OverlapPercent > overlaps[j].OverlapPercent
	})

	return overlaps
}

func printResults(results []TestResult, overlaps []OverlapResult) {
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
	fmt.Printf("Tests contributing zero unique coverage: %d\n", zeroCount)
	fmt.Println()

	// Show zero coverage tests with overlaps
	if zeroCount > 0 {
		fmt.Println("Tests with ZERO unique coverage and their overlaps:")
		fmt.Println("(Shows which test covers the most similar lines)")
		fmt.Println()

		// Create a map for quick lookup
		overlapMap := make(map[string]OverlapResult)
		for _, o := range overlaps {
			overlapMap[o.TestName] = o
		}

		// Print zero coverage tests with their overlaps
		zeroTests := []string{}
		for _, r := range results {
			if r.UniqueLines == 0 {
				zeroTests = append(zeroTests, r.Name)
			}
		}
		sort.Strings(zeroTests)

		for _, testName := range zeroTests {
			if overlap, ok := overlapMap[testName]; ok {
				fmt.Printf("  %-40s → %.0f%% overlap with %s (%d/%d lines)\n",
					testName, overlap.OverlapPercent, overlap.OverlapsWith,
					overlap.SharedLines, overlap.TotalLines)
			} else {
				// Check if test has actual coverage
				hasActualCoverage := false
				for _, r := range results {
					if r.Name == testName && len(r.ActualCoverage) > 0 {
						hasActualCoverage = true
						break
					}
				}
				if hasActualCoverage {
					fmt.Printf("  %-40s → no overlap found\n", testName)
				} else {
					fmt.Printf("  %-40s → covers no lines\n", testName)
				}
			}
		}
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

	fmt.Println("\nNote: Tests with 0 unique coverage contribution may still be valuable for:")
	fmt.Println("  - Ensuring error cases are handled correctly")
	fmt.Println("  - Testing output formatting")
	fmt.Println("  - Validating edge cases")
	fmt.Println("  - Preventing regressions")
}

func printJSONResults(results []TestResult, overlaps []OverlapResult) {
	type JSONOutput struct {
		Summary struct {
			TotalTests           int `json:"total_tests"`
			ZeroCoverageTests    int `json:"zero_coverage_tests"`
			TotalUniqueLines     int `json:"total_unique_lines"`
		} `json:"summary"`
		Tests []struct {
			Name           string  `json:"name"`
			UniqueLines    int     `json:"unique_lines"`
			ActualLines    int     `json:"actual_lines,omitempty"`
			OverlapsWith   string  `json:"overlaps_with,omitempty"`
			OverlapPercent float64 `json:"overlap_percent,omitempty"`
		} `json:"tests"`
	}

	var output JSONOutput
	output.Summary.TotalTests = len(results)
	
	// Create overlap map
	overlapMap := make(map[string]OverlapResult)
	for _, o := range overlaps {
		overlapMap[o.TestName] = o
	}
	
	// Count zero coverage tests and total unique lines
	for _, r := range results {
		if r.UniqueLines == 0 {
			output.Summary.ZeroCoverageTests++
		}
		output.Summary.TotalUniqueLines += r.UniqueLines
		
		test := struct {
			Name           string  `json:"name"`
			UniqueLines    int     `json:"unique_lines"`
			ActualLines    int     `json:"actual_lines,omitempty"`
			OverlapsWith   string  `json:"overlaps_with,omitempty"`
			OverlapPercent float64 `json:"overlap_percent,omitempty"`
		}{
			Name:        r.Name,
			UniqueLines: r.UniqueLines,
		}
		
		if len(r.ActualCoverage) > 0 {
			test.ActualLines = len(r.ActualCoverage)
		}
		
		if overlap, ok := overlapMap[r.Name]; ok {
			test.OverlapsWith = overlap.OverlapsWith
			test.OverlapPercent = overlap.OverlapPercent
		}
		
		output.Tests = append(output.Tests, test)
	}
	
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(output)
}

func printGroupedResults(results []TestResult) {
	type CoverageRange struct {
		Min, Max int
		Label    string
		Tests    []TestResult
	}
	
	ranges := []CoverageRange{
		{Min: 0, Max: 0, Label: "0 lines"},
		{Min: 1, Max: 5, Label: "1-5 lines"},
		{Min: 6, Max: 10, Label: "6-10 lines"},
		{Min: 11, Max: 50, Label: "11-50 lines"},
		{Min: 51, Max: 100, Label: "51-100 lines"},
		{Min: 101, Max: 999999, Label: "100+ lines"},
	}
	
	// Initialize ranges
	for i := range ranges {
		ranges[i].Tests = []TestResult{}
	}
	
	// Group tests into ranges
	for _, r := range results {
		for i, rng := range ranges {
			if r.UniqueLines >= rng.Min && r.UniqueLines <= rng.Max {
				ranges[i].Tests = append(ranges[i].Tests, r)
				break
			}
		}
	}
	
	fmt.Println("\n=========================================")
	fmt.Println("COVERAGE CONTRIBUTION GROUPS")
	fmt.Println("=========================================")
	fmt.Printf("Total tests analyzed: %d\n\n", len(results))
	
	for _, rng := range ranges {
		if len(rng.Tests) > 0 {
			percentage := float64(len(rng.Tests)) / float64(len(results)) * 100
			fmt.Printf("%-15s %4d tests (%5.1f%%)\n", rng.Label+":", len(rng.Tests), percentage)
			
			// Sort tests in this range
			sort.Slice(rng.Tests, func(i, j int) bool {
				if rng.Tests[i].UniqueLines == rng.Tests[j].UniqueLines {
					return rng.Tests[i].Name < rng.Tests[j].Name
				}
				return rng.Tests[i].UniqueLines > rng.Tests[j].UniqueLines
			})
			
			// Show up to 5 examples
			shown := 0
			for _, test := range rng.Tests {
				fmt.Printf("  %-40s %4d lines\n", test.Name, test.UniqueLines)
				shown++
				if shown >= 5 && len(rng.Tests) > 5 {
					fmt.Printf("  ... and %d more\n", len(rng.Tests)-5)
					break
				}
			}
			fmt.Println()
		}
	}
}
