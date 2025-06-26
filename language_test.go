package bkl_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gopatchy/bkl"
	"github.com/pelletier/go-toml/v2"
)

type TestCase struct {
	Description string
	Eval        []string
	Format      string
	Expected    string
	Files       map[string]string
	Error       string            // Expected error from evaluation
	RootPath    string            // Root path for restricting file access
	Env         map[string]string // Environment variables for the test
	Diff        bool              // Run diff operation instead of eval
	Intersect   bool              // Run intersect operation instead of eval
	Required    bool              // Run required operation instead of eval
	Benchmark   bool              // Run as benchmark test
}

type TestSuite map[string]TestCase

var (
	testFilter  = flag.String("test.filter", "", "Run only specified tests from tests.toml (comma-separated list)")
	testExclude = flag.String("test.exclude", "", "Exclude specified tests from tests.toml (comma-separated list)")
)

// runTestCase executes a single test case and returns the output and error
func runTestCase(testCase TestCase) ([]byte, error) {
	fsys := fstest.MapFS{}

	for filename, content := range testCase.Files {
		fsys[filename] = &fstest.MapFile{
			Data: []byte(content),
		}
	}

	rootPath := testCase.RootPath
	if rootPath == "" {
		rootPath = "/"
	}

	// Create a filesystem view rooted at the rootPath
	var testFS fs.FS = fsys
	if rootPath != "/" {
		var err error
		testFS, err = fs.Sub(fsys, rootPath)
		if err != nil {
			return nil, err
		}
	}

	p, err := bkl.New()
	if err != nil {
		return nil, err
	}

	var output []byte

	switch {
	case testCase.Required:
		// For required tests, we expect exactly 1 eval file
		if len(testCase.Eval) != 1 {
			return nil, fmt.Errorf("Required tests require exactly 1 eval file, got %d", len(testCase.Eval))
		}

		// Use the RequiredFile helper which matches bklr behavior
		requiredResult, err := p.RequiredFile(testFS, testCase.Eval[0])
		if err != nil {
			return nil, err
		}

		// Marshal the required result
		format := testCase.Format
		if format == "" {
			format = "yaml"
		}
		output, err = p.FormatOutput(requiredResult, format)
		if err != nil {
			return nil, err
		}

	case testCase.Intersect:
		// For intersect tests, we need at least 2 files
		if len(testCase.Eval) < 2 {
			return nil, fmt.Errorf("Intersect tests require at least 2 eval files, got %d", len(testCase.Eval))
		}

		// Use the IntersectFiles helper which matches bkli behavior
		intersectResult, err := p.IntersectFiles(testFS, testCase.Eval)
		if err != nil {
			return nil, err
		}

		// Marshal the intersect result
		format := testCase.Format
		if format == "" {
			format = "yaml"
		}
		output, err = p.FormatOutput(intersectResult, format)
		if err != nil {
			return nil, err
		}

	case testCase.Diff:
		// For diff tests, we expect exactly 2 eval files
		if len(testCase.Eval) != 2 {
			return nil, fmt.Errorf("Diff tests require exactly 2 eval files, got %d", len(testCase.Eval))
		}

		// Use the DiffFiles helper which matches bkld behavior
		diffResult, err := p.DiffFiles(testFS, testCase.Eval[0], testCase.Eval[1])
		if err != nil {
			return nil, err
		}

		// Marshal the diff result
		format := testCase.Format
		if format == "" {
			format = "yaml"
		}
		output, err = p.FormatOutput(diffResult, format)
		if err != nil {
			return nil, err
		}

	default:
		output, err = p.Evaluate(testFS, testCase.Eval, testCase.Format, rootPath, "/", testCase.Env)
	}

	return output, err
}

func TestLanguage(t *testing.T) {
	data, err := os.ReadFile("tests.toml")
	if err != nil {
		t.Fatalf("Failed to read tests.toml: %v", err)
	}

	var suite TestSuite
	err = toml.Unmarshal(data, &suite)
	if err != nil {
		t.Fatalf("Failed to parse tests.toml: %v", err)
	}

	// Parse filter list
	filterTests := map[string]bool{}
	if *testFilter != "" {
		for _, name := range strings.Split(*testFilter, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				if _, ok := suite[name]; !ok {
					t.Fatalf("Test %q not found in tests.toml", name)
				}
				filterTests[name] = true
			}
		}
	}

	// Parse exclude list
	excludeTests := map[string]bool{}
	if *testExclude != "" {
		for _, name := range strings.Split(*testExclude, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				if _, ok := suite[name]; !ok {
					t.Fatalf("Test %q not found in tests.toml", name)
				}
				excludeTests[name] = true
			}
		}
	}

	for testName, testCase := range suite {
		if len(filterTests) > 0 && !filterTests[testName] {
			continue
		}

		if excludeTests[testName] {
			continue
		}

		// Skip benchmark tests in regular test runs
		if testCase.Benchmark {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			output, err := runTestCase(testCase)

			if testCase.Error != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, but got no error", testCase.Error)
				}
				if !strings.Contains(err.Error(), testCase.Error) {
					t.Fatalf("Expected error containing %q, but got: %v", testCase.Error, err)
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

func BenchmarkLanguage(b *testing.B) {
	data, err := os.ReadFile("tests.toml")
	if err != nil {
		b.Fatalf("Failed to read tests.toml: %v", err)
	}

	var suite TestSuite
	err = toml.Unmarshal(data, &suite)
	if err != nil {
		b.Fatalf("Failed to parse tests.toml: %v", err)
	}

	for testName, testCase := range suite {
		if !testCase.Benchmark {
			continue
		}

		b.Run(testName, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				output, err := runTestCase(testCase)

				if err != nil && testCase.Error == "" {
					b.Fatalf("Unexpected error: %v", err)
				}

				// Don't validate output in benchmarks - we just care about performance
				_ = output
			}
		})
	}
}
