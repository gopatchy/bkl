package bkl_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopatchy/bkl"
)

func setupCLITestFiles(t *testing.T, files map[string]string) string {
	tmpDir := t.TempDir()

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

func addFormatArg(args []string, languages [][]any) []string {
	if len(languages) > 0 && len(languages[0]) > 1 {
		if format, ok := languages[0][1].(string); ok {
			return append(args, "--format", format)
		}
	}
	return args
}

func addRootPathArg(args []string, tmpDir, rootPath string) []string {
	if rootPath != "" {
		return append(args, "--root-path", filepath.Join(tmpDir, rootPath))
	}
	return args
}

func addSelectorArgs(args []string, selectors []string) []string {
	for _, sel := range selectors {
		args = append(args, "--selector", sel)
	}
	return args
}

func addSortArgs(args []string, sortPaths []string) []string {
	for _, sortPath := range sortPaths {
		args = append(args, "--sort", sortPath)
	}
	return args
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

func checkCLIOutput(t *testing.T, output []byte, expected string, removeInitialLines int) {
	expectedBytes := bytes.TrimSpace([]byte(expected))
	outputBytes := bytes.TrimSpace(output)

	if removeInitialLines > 0 {
		outputLines := bytes.Split(outputBytes, []byte("\n"))
		expectedLines := bytes.Split(expectedBytes, []byte("\n"))

		if len(outputLines) > removeInitialLines {
			outputBytes = bytes.Join(outputLines[removeInitialLines:], []byte("\n"))
		}
		if len(expectedLines) > removeInitialLines {
			expectedBytes = bytes.Join(expectedLines[removeInitialLines:], []byte("\n"))
		}
	}

	if !bytes.Equal(outputBytes, expectedBytes) {
		t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expectedBytes, outputBytes)
	}
}

func TestCLI(t *testing.T) {
	t.Parallel()

	tests, err := bkl.GetTests()
	if err != nil {
		t.Fatalf("Failed to get tests: %v", err)
	}

	for testName, testCase := range tests {
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
	files := map[string]string{}
	for _, input := range testCase.Evaluate.Inputs {
		files[input.Filename] = input.Code
	}

	tmpDir := setupCLITestFiles(t, files)

	var args []string
	args = addRootPathArg(args, tmpDir, testCase.Evaluate.Root)

	if len(testCase.Evaluate.Inputs) > 0 {
		lastInput := testCase.Evaluate.Inputs[len(testCase.Evaluate.Inputs)-1]
		args = append(args, filepath.Join(tmpDir, lastInput.Filename))
	}

	args = addFormatArg(args, testCase.Evaluate.Result.Languages)
	args = addSortArgs(args, testCase.Evaluate.Sort)

	output := executeCLICommand(t, "./cmd/bkl", args, testCase.Evaluate.Env, testCase.Evaluate.Errors)
	if output != nil {
		checkCLIOutput(t, output, testCase.Evaluate.Result.Code, 0)
	}
}

func runTestCLIRequired(t *testing.T, testCase *bkl.TestCase) {
	if len(testCase.Required.Inputs) != 1 {
		t.Fatalf("Required tests require exactly 1 eval file, got %d", len(testCase.Required.Inputs))
	}

	files := map[string]string{}
	for _, input := range testCase.Required.Inputs {
		files[input.Filename] = input.Code
	}

	tmpDir := setupCLITestFiles(t, files)

	var args []string
	args = addRootPathArg(args, tmpDir, testCase.Required.Root)

	args = append(args, filepath.Join(tmpDir, testCase.Required.Inputs[0].Filename))

	args = addFormatArg(args, testCase.Required.Result.Languages)

	output := executeCLICommand(t, "./cmd/bklr", args, testCase.Required.Env, testCase.Required.Errors)
	if output != nil {
		checkCLIOutput(t, output, testCase.Required.Result.Code, 0)
	}
}

func runTestCLIIntersect(t *testing.T, testCase *bkl.TestCase) {
	if len(testCase.Intersect.Inputs) < 2 {
		t.Fatalf("Intersect tests require at least 2 eval files, got %d", len(testCase.Intersect.Inputs))
	}

	files := map[string]string{}
	for _, input := range testCase.Intersect.Inputs {
		files[input.Filename] = input.Code
	}

	tmpDir := setupCLITestFiles(t, files)

	var args []string

	for _, input := range testCase.Intersect.Inputs {
		args = append(args, filepath.Join(tmpDir, input.Filename))
	}

	args = addFormatArg(args, testCase.Intersect.Result.Languages)
	args = addSelectorArgs(args, testCase.Intersect.Selector)

	output := executeCLICommand(t, "./cmd/bkli", args, nil, testCase.Intersect.Errors)
	if output != nil {
		checkCLIOutput(t, output, testCase.Intersect.Result.Code, 0)
	}
}

func runTestCLIDiff(t *testing.T, testCase *bkl.TestCase) {
	files := map[string]string{
		testCase.Diff.Base.Filename:   testCase.Diff.Base.Code,
		testCase.Diff.Target.Filename: testCase.Diff.Target.Code,
	}

	tmpDir := setupCLITestFiles(t, files)

	var args []string

	args = append(args, filepath.Join(tmpDir, testCase.Diff.Base.Filename))
	args = append(args, filepath.Join(tmpDir, testCase.Diff.Target.Filename))

	args = addFormatArg(args, testCase.Diff.Result.Languages)
	args = addSelectorArgs(args, testCase.Diff.Selector)

	output := executeCLICommand(t, "./cmd/bkld", args, nil, testCase.Diff.Errors)
	if output != nil {
		checkCLIOutput(t, output, testCase.Diff.Result.Code, 0)
	}
}

func runTestCLICompare(t *testing.T, testCase *bkl.TestCase) {
	files := map[string]string{
		testCase.Compare.Left.Filename:  testCase.Compare.Left.Code,
		testCase.Compare.Right.Filename: testCase.Compare.Right.Code,
	}

	tmpDir := setupCLITestFiles(t, files)

	var args []string

	args = append(args, filepath.Join(tmpDir, testCase.Compare.Left.Filename))
	args = append(args, filepath.Join(tmpDir, testCase.Compare.Right.Filename))

	args = addFormatArg(args, testCase.Compare.Result.Languages)
	args = addSortArgs(args, testCase.Compare.Sort)

	output := executeCLICommand(t, "./cmd/bklc", args, testCase.Compare.Env, nil)
	if output != nil {
		checkCLIOutput(t, output, testCase.Compare.Result.Code, 2)
	}
}
