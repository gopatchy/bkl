package bkl_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gopatchy/bkl"
)

var (
	testFilter  = flag.String("test.filter", "", "Run only specified tests from tests.toml (comma-separated list)")
	testExclude = flag.String("test.exclude", "", "Exclude specified tests from tests.toml (comma-separated list)")
	exportRegex = regexp.MustCompile(`#\s*export\s+([A-Z_]+)=(.*)`)
)

func runTestCase(testCase *bkl.TestCase) ([]byte, error) {
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

		output, err = bkl.Intersect(testFS, testCase.Eval, rootPath, rootPath, testCase.Selector, &testCase.Format, &testCase.Eval[0])
		if err != nil {
			return nil, err
		}

	case testCase.Diff:
		if len(testCase.Eval) != 2 {
			return nil, fmt.Errorf("Diff tests require exactly 2 eval files, got %d", len(testCase.Eval))
		}

		output, err = bkl.Diff(testFS, testCase.Eval[0], testCase.Eval[1], rootPath, rootPath, testCase.Selector, &testCase.Format, &testCase.Eval[0])
		if err != nil {
			return nil, err
		}

	default:
		output, err = bkl.Evaluate(testFS, testCase.Eval, rootPath, rootPath, testCase.Env, &testCase.Format, testCase.SortPath, &testCase.Eval[0])
	}

	return output, err
}

func TestBKL(t *testing.T) {
	t.Parallel()

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
	tests, err := bkl.GetTests()
	if err != nil {
		b.Fatalf("Failed to get tests: %v", err)
	}

	for testName, testCase := range tests {
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

			if testCase.Selector != "" && (testCase.Diff || testCase.Intersect) {
				args = append(args, "--selector", testCase.Selector)
			}

			if testCase.SortPath != "" {
				args = append(args, "--sort", testCase.SortPath)
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

func TestDocumentationExamples(t *testing.T) {
	t.Parallel()

	sections, err := bkl.GetDocSections()
	if err != nil {
		t.Fatalf("Failed to get doc sections: %v", err)
	}

	acceptableLanguages := []string{"yaml", "toml", "json"}

	for _, section := range sections {
	itemLoop:
		for itemIdx, item := range section.Items {
			if item.Example == nil {
				continue
			}

			example := item.Example
			testName := fmt.Sprintf("%s_item%d", section.ID, itemIdx)

			testCase := &bkl.TestCase{
				Description: fmt.Sprintf("Doc example from %s", section.Title),
				Files:       map[string]string{},
				Eval:        []string{},
				Env:         map[string]string{},
			}

			startIndex := 0
			if example.Operation == "convert" {
				if len(example.Layers) < 2 {
					continue itemLoop
				}
				startIndex = 1
			}

			for i, layer := range example.Layers {
				if example.Operation == "convert" && i < startIndex {
					continue
				}
				if len(layer.Languages) != 1 {
					continue itemLoop
				}
				layerLang := layer.Languages[0][1].(string)
				if !slices.Contains(acceptableLanguages, layerLang) {
					continue itemLoop
				}

				filename := fmt.Sprintf("file%d", i)
				if example.Operation == "evaluate" {
					if i == 0 {
						filename = "base"
					} else {
						filename = fmt.Sprintf("base.layer%d", i)
					}
				} else if example.Operation == "convert" {
					filename = fmt.Sprintf("file%d", i-startIndex+1)
				}

				filename += "." + layerLang

				testCase.Files[filename] = layer.Code

				lines := strings.Split(layer.Code, "\n")
				for _, line := range lines {
					if matches := exportRegex.FindStringSubmatch(line); matches != nil {
						testCase.Env[matches[1]] = matches[2]
					}
				}

				if example.Operation == "diff" || example.Operation == "intersect" {
					testCase.Eval = append(testCase.Eval, filename)
				} else {
					testCase.Eval = []string{filename}
				}
			}

			if example.Operation == "convert" {
				if len(example.Layers) > 0 && len(example.Layers[0].Languages) == 1 {
					testCase.Format = example.Layers[0].Languages[0][1].(string)
				} else {
					continue itemLoop
				}
			} else {
				if len(example.Result.Languages) != 1 {
					continue itemLoop
				}
				testCase.Format = example.Result.Languages[0][1].(string)
			}

			if !slices.Contains(acceptableLanguages, testCase.Format) {
				continue itemLoop
			}

			switch example.Operation {
			case "diff":
				testCase.Diff = true
			case "intersect":
				testCase.Intersect = true
			case "required":
				testCase.Required = true
			case "convert":
			}

			if section.ID == "bklr" {
				testCase.Required = true
			}
			t.Run(testName, func(t *testing.T) {
				output, err := runTestCase(testCase)

				expectedResult := strings.TrimSpace(example.Result.Code)
				if example.Operation == "convert" && len(example.Layers) > 0 {
					expectedResult = strings.TrimSpace(example.Layers[0].Code)
				}

				if expectedResult == "Error" {
					if err == nil {
						t.Errorf("Expected error but got none\nOutput: %s", output)
					}
					return
				}

				if err != nil {
					t.Errorf("Unexpected error: %v\nOutput: %s", err, output)
					return
				}

				if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(expectedResult))) {
					t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expectedResult, output)
				}
			})
		}
	}
}
