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

	rootPath := testCase.Root
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

	case testCase.Compare:
		if len(testCase.Eval) != 2 {
			return nil, fmt.Errorf("Compare tests require exactly 2 eval files, got %d", len(testCase.Eval))
		}

		result, err := bkl.Compare(testFS, testCase.Eval[0], testCase.Eval[1], rootPath, rootPath, testCase.Env, &testCase.Format, testCase.Sort)
		if err != nil {
			return nil, err
		}
		output = []byte(result.Diff)

	default:
		output, err = bkl.Evaluate(testFS, testCase.Eval, rootPath, rootPath, testCase.Env, &testCase.Format, testCase.Sort, &testCase.Eval[0])
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

	return tmpDir
}

func executeCLICommand(t *testing.T, cmdPath string, args []string, testCase *bkl.TestCase, tmpDir string) []byte {
	if testCase.Format != "" {
		args = append(args, "--format", testCase.Format)
	}

	if len(testCase.Selector) > 0 && (testCase.Diff || testCase.Intersect) {
		for _, selector := range testCase.Selector {
			args = append(args, "--selector", selector)
		}
	}

	if len(testCase.Sort) > 0 {
		for _, sortPath := range testCase.Sort {
			args = append(args, "--sort", sortPath)
		}
	}

	if testCase.Root != "" {
		args = append([]string{"--root-path", filepath.Join(tmpDir, testCase.Root)}, args...)
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

func runCLIEvaluateTest(t *testing.T, testName string, testCase *bkl.TestCase) {
	t.Run(testName, func(t *testing.T) {
		t.Parallel()
		tmpDir := setupCLITestFiles(t, testCase)

		var args []string
		for _, evalFile := range testCase.Eval {
			args = append(args, filepath.Join(tmpDir, evalFile))
		}

		output := executeCLICommand(t, "./cmd/bkl", args, testCase, tmpDir)
		if output != nil {
			checkCLIOutput(t, output, testCase.Expected, false)
		}
	})
}

func runCLIDiffTest(t *testing.T, testName string, testCase *bkl.TestCase) {
	t.Run(testName, func(t *testing.T) {
		t.Parallel()
		tmpDir := setupCLITestFiles(t, testCase)

		if len(testCase.Eval) != 2 {
			t.Fatalf("Diff tests require exactly 2 eval files, got %d", len(testCase.Eval))
		}

		args := []string{
			filepath.Join(tmpDir, testCase.Eval[0]),
			filepath.Join(tmpDir, testCase.Eval[1]),
		}

		output := executeCLICommand(t, "./cmd/bkld", args, testCase, tmpDir)
		if output != nil {
			checkCLIOutput(t, output, testCase.Expected, false)
		}
	})
}

func runCLIIntersectTest(t *testing.T, testName string, testCase *bkl.TestCase) {
	t.Run(testName, func(t *testing.T) {
		t.Parallel()
		tmpDir := setupCLITestFiles(t, testCase)

		if len(testCase.Eval) < 2 {
			t.Fatalf("Intersect tests require at least 2 eval files, got %d", len(testCase.Eval))
		}

		var args []string
		for _, evalFile := range testCase.Eval {
			args = append(args, filepath.Join(tmpDir, evalFile))
		}

		output := executeCLICommand(t, "./cmd/bkli", args, testCase, tmpDir)
		if output != nil {
			checkCLIOutput(t, output, testCase.Expected, false)
		}
	})
}

func runCLIRequiredTest(t *testing.T, testName string, testCase *bkl.TestCase) {
	t.Run(testName, func(t *testing.T) {
		t.Parallel()
		tmpDir := setupCLITestFiles(t, testCase)

		if len(testCase.Eval) != 1 {
			t.Fatalf("Required tests require exactly 1 eval file, got %d", len(testCase.Eval))
		}

		args := []string{filepath.Join(tmpDir, testCase.Eval[0])}

		output := executeCLICommand(t, "./cmd/bklr", args, testCase, tmpDir)
		if output != nil {
			checkCLIOutput(t, output, testCase.Expected, false)
		}
	})
}

func runCLICompareTest(t *testing.T, testName string, testCase *bkl.TestCase) {
	t.Run(testName, func(t *testing.T) {
		t.Parallel()
		tmpDir := setupCLITestFiles(t, testCase)

		if len(testCase.Eval) != 2 {
			t.Fatalf("Compare tests require exactly 2 eval files, got %d", len(testCase.Eval))
		}

		args := []string{
			filepath.Join(tmpDir, testCase.Eval[0]),
			filepath.Join(tmpDir, testCase.Eval[1]),
		}

		output := executeCLICommand(t, "./cmd/bklc", args, testCase, tmpDir)
		if output != nil {
			checkCLIOutput(t, output, testCase.Expected, true) // true to trim diff headers
		}
	})
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

		switch {
		case testCase.Compare:
			runCLICompareTest(t, testName, testCase)
		case testCase.Required:
			runCLIRequiredTest(t, testName, testCase)
		case testCase.Intersect:
			runCLIIntersectTest(t, testName, testCase)
		case testCase.Diff:
			runCLIDiffTest(t, testName, testCase)
		default:
			runCLIEvaluateTest(t, testName, testCase)
		}
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

func processEvaluateExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	if !validateLanguage(example.Evaluate.Inputs, acceptableLanguages) {
		return nil, false
	}

	testCase := &bkl.TestCase{
		Files: map[string]string{},
		Eval:  []string{},
		Env:   map[string]string{},
	}

	for i, layer := range example.Evaluate.Inputs {
		lang := layer.Languages[0][1].(string)
		filename := "base"
		if i > 0 {
			filename = fmt.Sprintf("base.layer%d", i)
		}
		filename += "." + lang

		testCase.Files[filename] = layer.Code
		testCase.Eval = []string{filename}

		for k, v := range extractEnvVars(layer.Code) {
			testCase.Env[k] = v
		}
	}

	if len(example.Evaluate.Result.Languages) != 1 {
		return nil, false
	}
	testCase.Format = example.Evaluate.Result.Languages[0][1].(string)
	testCase.Expected = example.Evaluate.Result.Code

	return testCase, true
}

func processConvertExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	if !validateLanguage([]bkl.DocLayer{example.Convert.To}, acceptableLanguages) {
		return nil, false
	}

	testCase := &bkl.TestCase{
		Files: map[string]string{},
		Eval:  []string{},
		Env:   map[string]string{},
	}

	lang := example.Convert.To.Languages[0][1].(string)
	filename := fmt.Sprintf("file.%s", lang)

	testCase.Files[filename] = example.Convert.To.Code
	testCase.Eval = []string{filename}

	for k, v := range extractEnvVars(example.Convert.To.Code) {
		testCase.Env[k] = v
	}

	if len(example.Convert.From.Languages) != 1 {
		return nil, false
	}
	testCase.Format = example.Convert.From.Languages[0][1].(string)
	testCase.Expected = example.Convert.From.Code

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

	testCase := &bkl.TestCase{
		Files:  map[string]string{},
		Eval:   []string{},
		Env:    map[string]string{},
		Format: lang,
	}

	filename := "good." + lang
	testCase.Files[filename] = goodLayer.Code
	testCase.Eval = []string{filename}
	testCase.Env = extractEnvVars(goodLayer.Code)
	testCase.Expected = goodLayer.Code

	return testCase, true
}

func processDiffExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	if !validateLanguage([]bkl.DocLayer{example.Diff.Base, example.Diff.Target}, acceptableLanguages) {
		return nil, false
	}

	testCase := &bkl.TestCase{
		Files: map[string]string{},
		Eval:  []string{},
		Env:   map[string]string{},
		Diff:  true,
	}

	baseLang := example.Diff.Base.Languages[0][1].(string)
	targetLang := example.Diff.Target.Languages[0][1].(string)

	testCase.Files["file0."+baseLang] = example.Diff.Base.Code
	testCase.Files["file1."+targetLang] = example.Diff.Target.Code
	testCase.Eval = []string{"file0." + baseLang, "file1." + targetLang}

	for k, v := range extractEnvVars(example.Diff.Base.Code) {
		testCase.Env[k] = v
	}
	for k, v := range extractEnvVars(example.Diff.Target.Code) {
		testCase.Env[k] = v
	}

	if len(example.Diff.Result.Languages) != 1 {
		return nil, false
	}
	testCase.Format = example.Diff.Result.Languages[0][1].(string)
	testCase.Expected = example.Diff.Result.Code

	return testCase, true
}

func processIntersectExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	if !validateLanguage(example.Intersect.Inputs, acceptableLanguages) {
		return nil, false
	}

	testCase := &bkl.TestCase{
		Files:     map[string]string{},
		Eval:      []string{},
		Env:       map[string]string{},
		Intersect: true,
	}

	for i, layer := range example.Intersect.Inputs {
		lang := layer.Languages[0][1].(string)
		filename := fmt.Sprintf("file%d.%s", i, lang)

		testCase.Files[filename] = layer.Code
		testCase.Eval = append(testCase.Eval, filename)

		for k, v := range extractEnvVars(layer.Code) {
			testCase.Env[k] = v
		}
	}

	if len(example.Intersect.Result.Languages) != 1 {
		return nil, false
	}
	testCase.Format = example.Intersect.Result.Languages[0][1].(string)
	testCase.Expected = example.Intersect.Result.Code

	return testCase, true
}

func processRequiredExample(example *bkl.DocExample, acceptableLanguages []string) (*bkl.TestCase, bool) {
	testCase, ok := processEvaluateExample(example, acceptableLanguages)
	if ok {
		testCase.Required = true
	}
	return testCase, ok
}

func runDocumentationTest(t *testing.T, testCase *bkl.TestCase, example *bkl.DocExample) {
	output, err := runTestCase(testCase)

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

	expectedResult := strings.TrimSpace(testCase.Expected)

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

			if section.ID == "bklr" {
				testCase.Required = true
			}

			if !slices.Contains(acceptableLanguages, testCase.Format) {
				continue
			}

			t.Run(testName, func(t *testing.T) {
				runDocumentationTest(t, testCase, example)
			})
		}
	}
}
