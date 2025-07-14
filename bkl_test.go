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

func runEvaluateTest(testCase *bkl.TestCase) ([]byte, error) {
	fsys := fstest.MapFS{}
	rootPath := testCase.Evaluate.Root
	if rootPath == "" {
		rootPath = "/"
	}

	var allFiles []string
	for _, input := range testCase.Evaluate.Inputs {
		fsys[input.Filename] = &fstest.MapFile{
			Data: []byte(input.Code),
		}
		allFiles = append(allFiles, input.Filename)
	}

	// Evaluate only the last file
	var evalFiles []string
	if len(allFiles) > 0 {
		evalFiles = []string{allFiles[len(allFiles)-1]}
	}

	var testFS fs.FS = fsys
	if rootPath != "/" {
		var err error
		testFS, err = fs.Sub(fsys, rootPath)
		if err != nil {
			return nil, err
		}
	}

	format := getFormat(testCase.Evaluate.Result.Languages)
	var firstFile *string
	if len(evalFiles) > 0 {
		firstFile = &evalFiles[0]
	}

	return bkl.Evaluate(testFS, evalFiles, rootPath, rootPath, testCase.Evaluate.Env, format, testCase.Evaluate.Sort, firstFile)
}

func runRequiredTest(testCase *bkl.TestCase) ([]byte, error) {
	fsys := fstest.MapFS{}
	rootPath := testCase.Required.Root
	if rootPath == "" {
		rootPath = "/"
	}

	var evalFiles []string
	for _, input := range testCase.Required.Inputs {
		fsys[input.Filename] = &fstest.MapFile{
			Data: []byte(input.Code),
		}
		if input.Code != "" {
			evalFiles = append(evalFiles, input.Filename)
		}
	}

	if len(evalFiles) != 1 {
		return nil, fmt.Errorf("Required tests require exactly 1 eval file, got %d", len(evalFiles))
	}

	var testFS fs.FS = fsys
	if rootPath != "/" {
		var err error
		testFS, err = fs.Sub(fsys, rootPath)
		if err != nil {
			return nil, err
		}
	}

	format := getFormat(testCase.Required.Result.Languages)
	firstFile := &evalFiles[0]

	return bkl.Required(testFS, evalFiles[0], rootPath, rootPath, format, firstFile)
}

func runIntersectTest(testCase *bkl.TestCase) ([]byte, error) {
	fsys := fstest.MapFS{}
	rootPath := "/"

	var evalFiles []string
	for _, input := range testCase.Intersect.Inputs {
		fsys[input.Filename] = &fstest.MapFile{
			Data: []byte(input.Code),
		}
		evalFiles = append(evalFiles, input.Filename)
	}

	if len(evalFiles) < 2 {
		return nil, fmt.Errorf("Intersect tests require at least 2 eval files, got %d", len(evalFiles))
	}

	format := getFormat(testCase.Intersect.Result.Languages)
	var firstFile *string
	if len(evalFiles) > 0 {
		firstFile = &evalFiles[0]
	}

	return bkl.Intersect(fsys, evalFiles, rootPath, rootPath, testCase.Intersect.Selector, format, firstFile)
}

func runDiffTest(testCase *bkl.TestCase) ([]byte, error) {
	fsys := fstest.MapFS{}
	rootPath := "/"

	fsys[testCase.Diff.Base.Filename] = &fstest.MapFile{
		Data: []byte(testCase.Diff.Base.Code),
	}
	fsys[testCase.Diff.Target.Filename] = &fstest.MapFile{
		Data: []byte(testCase.Diff.Target.Code),
	}

	format := getFormat(testCase.Diff.Result.Languages)
	firstFile := &testCase.Diff.Base.Filename

	return bkl.Diff(fsys, testCase.Diff.Base.Filename, testCase.Diff.Target.Filename, rootPath, rootPath, testCase.Diff.Selector, format, firstFile)
}

func runCompareTest(testCase *bkl.TestCase) ([]byte, error) {
	fsys := fstest.MapFS{}
	rootPath := "/"

	fsys[testCase.Compare.Left.Filename] = &fstest.MapFile{
		Data: []byte(testCase.Compare.Left.Code),
	}
	fsys[testCase.Compare.Right.Filename] = &fstest.MapFile{
		Data: []byte(testCase.Compare.Right.Code),
	}

	format := getFormat(testCase.Compare.Result.Languages)

	result, err := bkl.Compare(fsys, testCase.Compare.Left.Filename, testCase.Compare.Right.Filename, rootPath, rootPath, testCase.Compare.Env, format, testCase.Compare.Sort)
	if err != nil {
		return nil, err
	}
	return []byte(result.Diff), nil
}

func getFormat(languages [][]any) *string {
	if len(languages) > 0 && len(languages[0]) > 1 {
		if format, ok := languages[0][1].(string); ok {
			return &format
		}
	}
	return nil
}

func runTestBKLEvaluate(t *testing.T, testCase *bkl.TestCase) {
	output, err := runEvaluateTest(testCase)

	if len(testCase.Evaluate.Errors) > 0 {
		if err == nil {
			t.Fatalf("Expected error containing one of %v, but got no error", testCase.Evaluate.Errors)
		}

		errorFound := false
		for _, expectedError := range testCase.Evaluate.Errors {
			if strings.Contains(err.Error(), expectedError) {
				errorFound = true
				break
			}
		}

		if !errorFound {
			t.Fatalf("Expected error containing one of %v, but got: %v", testCase.Evaluate.Errors, err)
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := testCase.Evaluate.Result.Code
	if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(expected))) {
		t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expected, output)
	}
}

func runTestBKLRequired(t *testing.T, testCase *bkl.TestCase) {
	output, err := runRequiredTest(testCase)

	if len(testCase.Required.Errors) > 0 {
		if err == nil {
			t.Fatalf("Expected error containing one of %v, but got no error", testCase.Required.Errors)
		}

		errorFound := false
		for _, expectedError := range testCase.Required.Errors {
			if strings.Contains(err.Error(), expectedError) {
				errorFound = true
				break
			}
		}

		if !errorFound {
			t.Fatalf("Expected error containing one of %v, but got: %v", testCase.Required.Errors, err)
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := testCase.Required.Result.Code
	if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(expected))) {
		t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expected, output)
	}
}

func runTestBKLIntersect(t *testing.T, testCase *bkl.TestCase) {
	output, err := runIntersectTest(testCase)

	if len(testCase.Intersect.Errors) > 0 {
		if err == nil {
			t.Fatalf("Expected error containing one of %v, but got no error", testCase.Intersect.Errors)
		}

		errorFound := false
		for _, expectedError := range testCase.Intersect.Errors {
			if strings.Contains(err.Error(), expectedError) {
				errorFound = true
				break
			}
		}

		if !errorFound {
			t.Fatalf("Expected error containing one of %v, but got: %v", testCase.Intersect.Errors, err)
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := testCase.Intersect.Result.Code
	if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(expected))) {
		t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expected, output)
	}
}

func runTestBKLDiff(t *testing.T, testCase *bkl.TestCase) {
	output, err := runDiffTest(testCase)

	if len(testCase.Diff.Errors) > 0 {
		if err == nil {
			t.Fatalf("Expected error containing one of %v, but got no error", testCase.Diff.Errors)
		}

		errorFound := false
		for _, expectedError := range testCase.Diff.Errors {
			if strings.Contains(err.Error(), expectedError) {
				errorFound = true
				break
			}
		}

		if !errorFound {
			t.Fatalf("Expected error containing one of %v, but got: %v", testCase.Diff.Errors, err)
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := testCase.Diff.Result.Code
	if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(expected))) {
		t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expected, output)
	}
}

func runTestBKLCompare(t *testing.T, testCase *bkl.TestCase) {
	output, err := runCompareTest(testCase)
	// DocCompare doesn't have Errors field, so no error checking
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := testCase.Compare.Result.Code
	if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(expected))) {
		t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expected, output)
	}
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
			switch {
			case testCase.Evaluate != nil:
				runTestBKLEvaluate(t, testCase)
			case testCase.Required != nil:
				runTestBKLRequired(t, testCase)
			case testCase.Intersect != nil:
				runTestBKLIntersect(t, testCase)
			case testCase.Diff != nil:
				runTestBKLDiff(t, testCase)
			case testCase.Compare != nil:
				runTestBKLCompare(t, testCase)
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

		switch {
		case testCase.Evaluate != nil:
			b.Run(testName, func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					output, err := runEvaluateTest(testCase)
					if err != nil && len(testCase.Evaluate.Errors) == 0 {
						b.Fatalf("Unexpected error: %v", err)
					}
					_ = output
				}
			})
		case testCase.Required != nil:
			b.Run(testName, func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					output, err := runRequiredTest(testCase)
					if err != nil && len(testCase.Required.Errors) == 0 {
						b.Fatalf("Unexpected error: %v", err)
					}
					_ = output
				}
			})
		case testCase.Intersect != nil:
			b.Run(testName, func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					output, err := runIntersectTest(testCase)
					if err != nil && len(testCase.Intersect.Errors) == 0 {
						b.Fatalf("Unexpected error: %v", err)
					}
					_ = output
				}
			})
		case testCase.Diff != nil:
			b.Run(testName, func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					output, err := runDiffTest(testCase)
					if err != nil && len(testCase.Diff.Errors) == 0 {
						b.Fatalf("Unexpected error: %v", err)
					}
					_ = output
				}
			})
		case testCase.Compare != nil:
			b.Run(testName, func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					output, err := runCompareTest(testCase)
					if err != nil {
						b.Fatalf("Unexpected error: %v", err)
					}
					_ = output
				}
			})
		}
	}
}

func getFilteredTests(t *testing.T) (map[string]*bkl.TestCase, map[string]bool, map[string]bool) {
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

	return tests, filterTests, excludeTests
}

func setupCLITestFiles(t *testing.T, testCase *bkl.TestCase) string {
	tmpDir := t.TempDir()

	// Extract files from operation-specific structure
	files := make(map[string]string)
	switch {
	case testCase.Required != nil:
		for _, input := range testCase.Required.Inputs {
			files[input.Filename] = input.Code
		}
	case testCase.Intersect != nil:
		for _, input := range testCase.Intersect.Inputs {
			files[input.Filename] = input.Code
		}
	case testCase.Diff != nil:
		files[testCase.Diff.Base.Filename] = testCase.Diff.Base.Code
		files[testCase.Diff.Target.Filename] = testCase.Diff.Target.Code
	case testCase.Compare != nil:
		files[testCase.Compare.Left.Filename] = testCase.Compare.Left.Code
		files[testCase.Compare.Right.Filename] = testCase.Compare.Right.Code
	case testCase.Evaluate != nil:
		for _, input := range testCase.Evaluate.Inputs {
			files[input.Filename] = input.Code
		}
	}

	for filename, content := range files {
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

	return tmpDir
}

func executeCLICommand(t *testing.T, cmdPath string, args []string, env map[string]string, expectedErrors []string) []byte {
	cmdArgs := append([]string{"run", cmdPath}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = "."

	if env != nil {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	output, err := cmd.CombinedOutput()

	if len(expectedErrors) > 0 {
		if err == nil {
			t.Fatalf("Expected error containing one of %v, but got no error", expectedErrors)
		}

		errorFound := false
		for _, expectedError := range expectedErrors {
			if strings.Contains(string(output), expectedError) || strings.Contains(err.Error(), expectedError) {
				errorFound = true
				break
			}
		}

		if !errorFound {
			t.Fatalf("Expected error containing one of %v, but got: %v\nOutput: %s", expectedErrors, err, output)
		}
		return nil
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v\nOutput: %s", err, output)
	}

	return output
}

func checkCLIOutput(t *testing.T, output []byte, expected string, trimDiffHeaders bool) {
	expectedBytes := bytes.TrimSpace([]byte(expected))
	outputBytes := bytes.TrimSpace(output)

	if trimDiffHeaders {
		// Split by newlines and remove first two lines if they exist
		outputLines := bytes.Split(outputBytes, []byte("\n"))
		expectedLines := bytes.Split(expectedBytes, []byte("\n"))

		if len(outputLines) > 2 {
			outputBytes = bytes.Join(outputLines[2:], []byte("\n"))
		}
		if len(expectedLines) > 2 {
			expectedBytes = bytes.Join(expectedLines[2:], []byte("\n"))
		}
	}

	if !bytes.Equal(outputBytes, expectedBytes) {
		t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expectedBytes, outputBytes)
	}
}

func TestCLI(t *testing.T) {
	t.Parallel()

	tests, filterTests, excludeTests := getFilteredTests(t)

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
			switch {
			case testCase.Evaluate != nil:
				runTestCLIEvaluate(t, testCase)
			case testCase.Required != nil:
				runTestCLIRequired(t, testCase)
			case testCase.Intersect != nil:
				runTestCLIIntersect(t, testCase)
			case testCase.Diff != nil:
				runTestCLIDiff(t, testCase)
			case testCase.Compare != nil:
				runTestCLICompare(t, testCase)
			}
		})
	}
}

func runTestCLIEvaluate(t *testing.T, testCase *bkl.TestCase) {
	tmpDir := setupCLITestFiles(t, testCase)

	var args []string
	if testCase.Evaluate != nil {
		// Add root path if specified
		if testCase.Evaluate.Root != "" {
			args = append(args, "--root-path", filepath.Join(tmpDir, testCase.Evaluate.Root))
		}

		// Add files - only evaluate the last file to match TestBKL behavior
		if len(testCase.Evaluate.Inputs) > 0 {
			lastInput := testCase.Evaluate.Inputs[len(testCase.Evaluate.Inputs)-1]
			args = append(args, filepath.Join(tmpDir, lastInput.Filename))
		}

		// Add format if specified
		format := getFormat(testCase.Evaluate.Result.Languages)
		if format != nil {
			args = append(args, "--format", *format)
		}

		// Add sort if specified
		for _, sortPath := range testCase.Evaluate.Sort {
			args = append(args, "--sort", sortPath)
		}
	}

	output := executeCLICommand(t, "./cmd/bkl", args, testCase.Evaluate.Env, testCase.Evaluate.Errors)
	if output != nil {
		var expected string
		if testCase.Evaluate != nil {
			expected = testCase.Evaluate.Result.Code
		}
		checkCLIOutput(t, output, expected, false)
	}
}

func runTestCLIRequired(t *testing.T, testCase *bkl.TestCase) {
	tmpDir := setupCLITestFiles(t, testCase)

	if testCase.Required == nil {
		t.Fatalf("Expected Required operation")
	}

	if len(testCase.Required.Inputs) != 1 {
		t.Fatalf("Required tests require exactly 1 eval file, got %d", len(testCase.Required.Inputs))
	}

	var args []string

	// Add root path if specified
	if testCase.Required.Root != "" {
		args = append(args, "--root-path", filepath.Join(tmpDir, testCase.Required.Root))
	}

	// Add file
	args = append(args, filepath.Join(tmpDir, testCase.Required.Inputs[0].Filename))

	// Add format if specified
	format := getFormat(testCase.Required.Result.Languages)
	if format != nil {
		args = append(args, "--format", *format)
	}

	output := executeCLICommand(t, "./cmd/bklr", args, testCase.Required.Env, testCase.Required.Errors)
	if output != nil {
		checkCLIOutput(t, output, testCase.Required.Result.Code, false)
	}
}

func runTestCLIIntersect(t *testing.T, testCase *bkl.TestCase) {
	tmpDir := setupCLITestFiles(t, testCase)

	if testCase.Intersect == nil {
		t.Fatalf("Expected Intersect operation")
	}

	if len(testCase.Intersect.Inputs) < 2 {
		t.Fatalf("Intersect tests require at least 2 eval files, got %d", len(testCase.Intersect.Inputs))
	}

	var args []string

	// Add files
	for _, input := range testCase.Intersect.Inputs {
		args = append(args, filepath.Join(tmpDir, input.Filename))
	}

	// Add format if specified
	format := getFormat(testCase.Intersect.Result.Languages)
	if format != nil {
		args = append(args, "--format", *format)
	}

	// Add selectors if specified
	for _, sel := range testCase.Intersect.Selector {
		args = append(args, "--selector", sel)
	}

	output := executeCLICommand(t, "./cmd/bkli", args, nil, testCase.Intersect.Errors)
	if output != nil {
		checkCLIOutput(t, output, testCase.Intersect.Result.Code, false)
	}
}

func runTestCLIDiff(t *testing.T, testCase *bkl.TestCase) {
	tmpDir := setupCLITestFiles(t, testCase)

	if testCase.Diff == nil {
		t.Fatalf("Expected Diff operation")
	}

	var args []string

	// Add files
	args = append(args, filepath.Join(tmpDir, testCase.Diff.Base.Filename))
	args = append(args, filepath.Join(tmpDir, testCase.Diff.Target.Filename))

	// Add format if specified
	format := getFormat(testCase.Diff.Result.Languages)
	if format != nil {
		args = append(args, "--format", *format)
	}

	// Add selectors if specified
	for _, sel := range testCase.Diff.Selector {
		args = append(args, "--selector", sel)
	}

	output := executeCLICommand(t, "./cmd/bkld", args, nil, testCase.Diff.Errors)
	if output != nil {
		checkCLIOutput(t, output, testCase.Diff.Result.Code, false)
	}
}

func runTestCLICompare(t *testing.T, testCase *bkl.TestCase) {
	tmpDir := setupCLITestFiles(t, testCase)

	if testCase.Compare == nil {
		t.Fatalf("Expected Compare operation")
	}

	var args []string

	// Add files
	args = append(args, filepath.Join(tmpDir, testCase.Compare.Left.Filename))
	args = append(args, filepath.Join(tmpDir, testCase.Compare.Right.Filename))

	// Add format if specified
	format := getFormat(testCase.Compare.Result.Languages)
	if format != nil {
		args = append(args, "--format", *format)
	}

	// Add sort if specified
	for _, sortPath := range testCase.Compare.Sort {
		args = append(args, "--sort", sortPath)
	}

	// Note: Compare doesn't have Errors field
	output := executeCLICommand(t, "./cmd/bklc", args, testCase.Compare.Env, nil)
	if output != nil {
		checkCLIOutput(t, output, testCase.Compare.Result.Code, true) // true to trim diff headers
	}
}

func extractEnvVars(code string) map[string]string {
	env := make(map[string]string)
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		if matches := exportRegex.FindStringSubmatch(line); matches != nil {
			env[matches[1]] = matches[2]
		}
	}
	return env
}

func validateLanguage(layers []bkl.DocLayer, acceptableLanguages []string) bool {
	for _, layer := range layers {
		if len(layer.Languages) != 1 {
			return false
		}
		lang := layer.Languages[0][1].(string)
		if !slices.Contains(acceptableLanguages, lang) {
			return false
		}
	}
	return true
}

func validateLanguagePointers(layers []*bkl.DocLayer, acceptableLanguages []string) bool {
	for _, layer := range layers {
		if len(layer.Languages) != 1 {
			return false
		}
		lang := layer.Languages[0][1].(string)
		if !slices.Contains(acceptableLanguages, lang) {
			return false
		}
	}
	return true
}

func processEvaluateExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	if !validateLanguagePointers(example.Evaluate.Inputs, acceptableLanguages) {
		return nil, false
	}

	// Build inputs with proper filenames
	inputs := make([]*bkl.DocLayer, 0, len(example.Evaluate.Inputs))
	env := map[string]string{}

	for i, layer := range example.Evaluate.Inputs {
		lang := layer.Languages[0][1].(string)
		filename := "base"
		if i > 0 {
			filename = fmt.Sprintf("base.layer%d", i)
		}
		filename += "." + lang

		inputs = append(inputs, &bkl.DocLayer{
			Filename:  filename,
			Code:      layer.Code,
			Languages: layer.Languages,
		})

		for k, v := range extractEnvVars(layer.Code) {
			env[k] = v
		}
	}

	if len(example.Evaluate.Result.Languages) != 1 {
		return nil, false
	}

	testCase := &bkl.TestCase{
		Description: "Doc example",
		Evaluate: &bkl.DocEvaluate{
			Inputs: inputs,
			Result: example.Evaluate.Result,
			Env:    env,
		},
	}

	return testCase, true
}

func processConvertExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	if !validateLanguage([]bkl.DocLayer{example.Convert.To}, acceptableLanguages) {
		return nil, false
	}

	lang := example.Convert.To.Languages[0][1].(string)
	filename := fmt.Sprintf("file.%s", lang)

	env := map[string]string{}
	for k, v := range extractEnvVars(example.Convert.To.Code) {
		env[k] = v
	}

	if len(example.Convert.From.Languages) != 1 {
		return nil, false
	}

	testCase := &bkl.TestCase{
		Description: "Doc example",
		Evaluate: &bkl.DocEvaluate{
			Inputs: []*bkl.DocLayer{{
				Filename:  filename,
				Code:      example.Convert.To.Code,
				Languages: example.Convert.To.Languages,
			}},
			Result: example.Convert.From,
			Env:    env,
		},
	}

	return testCase, true
}

func processFixitExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	goodLayer := example.Fixit.Good

	if len(goodLayer.Languages) != 1 {
		return nil, false
	}

	lang := goodLayer.Languages[0][1].(string)
	if !slices.Contains(acceptableLanguages, lang) {
		return nil, false
	}

	filename := "good." + lang

	testCase := &bkl.TestCase{
		Description: "Doc example",
		Evaluate: &bkl.DocEvaluate{
			Inputs: []*bkl.DocLayer{{
				Filename:  filename,
				Code:      goodLayer.Code,
				Languages: goodLayer.Languages,
			}},
			Result: goodLayer,
			Env:    extractEnvVars(goodLayer.Code),
		},
	}

	return testCase, true
}

func processDiffExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	if !validateLanguage([]bkl.DocLayer{example.Diff.Base, example.Diff.Target}, acceptableLanguages) {
		return nil, false
	}

	baseLang := example.Diff.Base.Languages[0][1].(string)
	targetLang := example.Diff.Target.Languages[0][1].(string)

	// Set filenames
	base := example.Diff.Base
	base.Filename = "file0." + baseLang

	target := example.Diff.Target
	target.Filename = "file1." + targetLang

	if len(example.Diff.Result.Languages) != 1 {
		return nil, false
	}

	testCase := &bkl.TestCase{
		Description: "Doc example",
		Diff: &bkl.DocDiff{
			Base:   base,
			Target: target,
			Result: example.Diff.Result,
		},
	}

	return testCase, true
}

func processIntersectExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	if !validateLanguagePointers(example.Intersect.Inputs, acceptableLanguages) {
		return nil, false
	}

	// Set filenames for inputs
	inputs := make([]*bkl.DocLayer, len(example.Intersect.Inputs))
	for i, layer := range example.Intersect.Inputs {
		lang := layer.Languages[0][1].(string)
		filename := fmt.Sprintf("file%d.%s", i, lang)

		inputs[i] = &bkl.DocLayer{
			Filename:  filename,
			Code:      layer.Code,
			Languages: layer.Languages,
		}
	}

	if len(example.Intersect.Result.Languages) != 1 {
		return nil, false
	}

	testCase := &bkl.TestCase{
		Description: "Doc example",
		Intersect: &bkl.DocIntersect{
			Inputs: inputs,
			Result: example.Intersect.Result,
		},
	}

	return testCase, true
}

func processRequiredExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	testCase, ok := processEvaluateExample(example, acceptableLanguages)
	if ok && testCase.Evaluate != nil {
		// Convert Evaluate to Required
		testCase.Required = testCase.Evaluate
		testCase.Evaluate = nil
	}
	return testCase, ok
}

func runDocumentationTest(t *testing.T, testCase *bkl.TestCase, example *bkl.DocExample) {
	var output []byte
	var err error

	switch {
	case testCase.Evaluate != nil:
		output, err = runEvaluateTest(testCase)
	case testCase.Required != nil:
		output, err = runRequiredTest(testCase)
	case testCase.Intersect != nil:
		output, err = runIntersectTest(testCase)
	case testCase.Diff != nil:
		output, err = runDiffTest(testCase)
	case testCase.Compare != nil:
		output, err = runCompareTest(testCase)
	default:
		t.Errorf("Test case has no operation defined")
		return
	}

	if example.Fixit != nil {
		if err != nil {
			t.Errorf("Fixit good code failed to evaluate: %v\nOutput: %s", err, output)
			return
		}

		if example.Fixit.Original.Code != "" {
			originalCode := strings.TrimSpace(example.Fixit.Original.Code)
			actualOutput := strings.TrimSpace(string(output))

			if actualOutput != originalCode {
				t.Errorf("Fixit good code output doesn't match original\nOriginal:\n%s\nActual output:\n%s", originalCode, actualOutput)
			}
		}
		return
	}

	// Get expected result from operation-specific structure
	var expectedResult string
	switch {
	case testCase.Required != nil:
		expectedResult = strings.TrimSpace(testCase.Required.Result.Code)
	case testCase.Intersect != nil:
		expectedResult = strings.TrimSpace(testCase.Intersect.Result.Code)
	case testCase.Diff != nil:
		expectedResult = strings.TrimSpace(testCase.Diff.Result.Code)
	case testCase.Evaluate != nil:
		expectedResult = strings.TrimSpace(testCase.Evaluate.Result.Code)
	default:
		t.Errorf("Test case has no operation defined")
		return
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
}

func TestDocumentationExamples(t *testing.T) {
	t.Parallel()

	sections, err := bkl.GetDocSections()
	if err != nil {
		t.Fatalf("Failed to get doc sections: %v", err)
	}

	acceptableLanguages := []string{"yaml", "toml", "json"}

	for _, section := range sections {
		for itemIdx, item := range section.Items {
			if item.Example == nil {
				continue
			}

			example := item.Example
			testName := fmt.Sprintf("%s_item%d", section.ID, itemIdx)

			var testCase *bkl.TestCase
			var ok bool

			switch {
			case example.Evaluate != nil:
				testCase, ok = processEvaluateExample(example, acceptableLanguages)
			case example.Convert != nil:
				testCase, ok = processConvertExample(example, acceptableLanguages)
			case example.Fixit != nil:
				testCase, ok = processFixitExample(example, acceptableLanguages)
			case example.Diff != nil:
				testCase, ok = processDiffExample(example, acceptableLanguages)
			case example.Intersect != nil:
				testCase, ok = processIntersectExample(example, acceptableLanguages)
			case example.Compare != nil:
				continue
			default:
				continue
			}

			if !ok {
				continue
			}

			testCase.Description = fmt.Sprintf("Doc example from %s", section.Title)

			if section.ID == "bklr" && testCase.Evaluate != nil {
				// Convert Evaluate to Required
				testCase.Required = testCase.Evaluate
				testCase.Evaluate = nil
			}

			// Check format from operation-specific structure
			var actualFormat *string
			switch {
			case testCase.Required != nil:
				actualFormat = getFormat(testCase.Required.Result.Languages)
			case testCase.Intersect != nil:
				actualFormat = getFormat(testCase.Intersect.Result.Languages)
			case testCase.Diff != nil:
				actualFormat = getFormat(testCase.Diff.Result.Languages)
			case testCase.Evaluate != nil:
				actualFormat = getFormat(testCase.Evaluate.Result.Languages)
			}

			if actualFormat == nil || !slices.Contains(acceptableLanguages, *actualFormat) {
				continue
			}

			t.Run(testName, func(t *testing.T) {
				runDocumentationTest(t, testCase, example)
			})
		}
	}
}
