package bkl_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
	Errors      []string          // Expected errors from evaluation (one must match)
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

	var testFS fs.FS = fsys
	if rootPath != "/" {
		var err error
		testFS, err = fs.Sub(fsys, rootPath)
		if err != nil {
			return nil, err
		}
	}

	var output []byte
	var err error

	switch {
	case testCase.Required:
		if len(testCase.Eval) != 1 {
			return nil, fmt.Errorf("Required tests require exactly 1 eval file, got %d", len(testCase.Eval))
		}

		output, err = bkl.Required(testFS, testCase.Eval[0], rootPath, rootPath, &testCase.Format, &testCase.Eval[0])
		if err != nil {
			return nil, err
		}

	case testCase.Intersect:
		if len(testCase.Eval) < 2 {
			return nil, fmt.Errorf("Intersect tests require at least 2 eval files, got %d", len(testCase.Eval))
		}

		output, err = bkl.Intersect(testFS, testCase.Eval, rootPath, rootPath, &testCase.Format, &testCase.Eval[0])
		if err != nil {
			return nil, err
		}

	case testCase.Diff:
		if len(testCase.Eval) != 2 {
			return nil, fmt.Errorf("Diff tests require exactly 2 eval files, got %d", len(testCase.Eval))
		}

		output, err = bkl.Diff(testFS, testCase.Eval[0], testCase.Eval[1], rootPath, rootPath, &testCase.Format, &testCase.Eval[0])
		if err != nil {
			return nil, err
		}

	default:
		output, err = bkl.Evaluate(testFS, testCase.Eval, rootPath, rootPath, testCase.Env, &testCase.Format, &testCase.Eval[0])
	}

	return output, err
}

func TestBKL(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("tests.toml")
	if err != nil {
		t.Fatalf("Failed to read tests.toml: %v", err)
	}

	var suite TestSuite
	err = toml.Unmarshal(data, &suite)
	if err != nil {
		t.Fatalf("Failed to parse tests.toml: %v", err)
	}

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

		if testCase.Benchmark {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			output, err := runTestCase(testCase)

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

func BenchmarkBKL(b *testing.B) {
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

				if err != nil && len(testCase.Errors) == 0 {
					b.Fatalf("Unexpected error: %v", err)
				}

				_ = output
			}
		})
	}
}

func TestCLI(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("tests.toml")
	if err != nil {
		t.Fatalf("Failed to read tests.toml: %v", err)
	}

	var suite TestSuite
	err = toml.Unmarshal(data, &suite)
	if err != nil {
		t.Fatalf("Failed to parse tests.toml: %v", err)
	}

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

		if testCase.Benchmark {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()

			for filename, content := range testCase.Files {
				fullPath := filepath.Join(tmpDir, filename)
				dir := filepath.Dir(fullPath)
				if dir != tmpDir && dir != "." {
					err := os.MkdirAll(dir, 0o755)
					if err != nil {
						t.Fatalf("Failed to create directory %s: %v", dir, err)
					}
				}
				err := os.WriteFile(fullPath, []byte(content), 0o644)
				if err != nil {
					t.Fatalf("Failed to write test file %s: %v", filename, err)
				}
			}

			var cmdPath string
			var args []string

			switch {
			case testCase.Diff:
				cmdPath = "./cmd/bkld"
				if len(testCase.Eval) != 2 {
					t.Fatalf("Diff tests require exactly 2 eval files, got %d", len(testCase.Eval))
				}
				args = append(args, filepath.Join(tmpDir, testCase.Eval[0]))
				args = append(args, filepath.Join(tmpDir, testCase.Eval[1]))
			case testCase.Intersect:
				cmdPath = "./cmd/bkli"
				if len(testCase.Eval) < 2 {
					t.Fatalf("Intersect tests require at least 2 eval files, got %d", len(testCase.Eval))
				}
				for _, evalFile := range testCase.Eval {
					args = append(args, filepath.Join(tmpDir, evalFile))
				}
			case testCase.Required:
				cmdPath = "./cmd/bklr"
				if len(testCase.Eval) != 1 {
					t.Fatalf("Required tests require exactly 1 eval file, got %d", len(testCase.Eval))
				}
				args = append(args, filepath.Join(tmpDir, testCase.Eval[0]))
			default:
				cmdPath = "./cmd/bkl"
				for _, evalFile := range testCase.Eval {
					args = append(args, filepath.Join(tmpDir, evalFile))
				}
			}

			if testCase.Format != "" {
				args = append(args, "--format", testCase.Format)
			}

			if testCase.RootPath != "" {
				args = append([]string{"--root-path", filepath.Join(tmpDir, testCase.RootPath)}, args...)
			}

			cmdArgs := append([]string{"run", cmdPath}, args...)
			cmd := exec.Command("go", cmdArgs...)
			cmd.Dir = "."

			if testCase.Env != nil {
				cmd.Env = os.Environ()
				for k, v := range testCase.Env {
					cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
				}
			}

			output, err := cmd.CombinedOutput()

			if len(testCase.Errors) > 0 {
				if err == nil {
					t.Fatalf("Expected error containing one of %v, but got no error", testCase.Errors)
				}

				errorFound := false
				for _, expectedError := range testCase.Errors {
					if strings.Contains(string(output), expectedError) || strings.Contains(err.Error(), expectedError) {
						errorFound = true
						break
					}
				}

				if !errorFound {
					t.Fatalf("Expected error containing one of %v, but got: %v\nOutput: %s", testCase.Errors, err, output)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v\nOutput: %s", err, output)
			}

			if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(testCase.Expected))) {
				t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", testCase.Expected, output)
			}
		})
	}
}
