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

func setupCLITestFiles(t *testing.T, testCase *bkl.TestCase) string {
	tmpDir := t.TempDir()

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
		if testCase.Evaluate.Root != "" {
			args = append(args, "--root-path", filepath.Join(tmpDir, testCase.Evaluate.Root))
		}

		if len(testCase.Evaluate.Inputs) > 0 {
			lastInput := testCase.Evaluate.Inputs[len(testCase.Evaluate.Inputs)-1]
			args = append(args, filepath.Join(tmpDir, lastInput.Filename))
		}

		format := getFormat(testCase.Evaluate.Result.Languages)
		if format != nil {
			args = append(args, "--format", *format)
		}

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

	if testCase.Required.Root != "" {
		args = append(args, "--root-path", filepath.Join(tmpDir, testCase.Required.Root))
	}

	args = append(args, filepath.Join(tmpDir, testCase.Required.Inputs[0].Filename))

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

	for _, input := range testCase.Intersect.Inputs {
		args = append(args, filepath.Join(tmpDir, input.Filename))
	}

	format := getFormat(testCase.Intersect.Result.Languages)
	if format != nil {
		args = append(args, "--format", *format)
	}

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

	args = append(args, filepath.Join(tmpDir, testCase.Diff.Base.Filename))
	args = append(args, filepath.Join(tmpDir, testCase.Diff.Target.Filename))

	format := getFormat(testCase.Diff.Result.Languages)
	if format != nil {
		args = append(args, "--format", *format)
	}

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

	args = append(args, filepath.Join(tmpDir, testCase.Compare.Left.Filename))
	args = append(args, filepath.Join(tmpDir, testCase.Compare.Right.Filename))

	format := getFormat(testCase.Compare.Result.Languages)
	if format != nil {
		args = append(args, "--format", *format)
	}

	for _, sortPath := range testCase.Compare.Sort {
		args = append(args, "--sort", sortPath)
	}

	output := executeCLICommand(t, "./cmd/bklc", args, testCase.Compare.Env, nil)
	if output != nil {
		checkCLIOutput(t, output, testCase.Compare.Result.Code, true) // true to trim diff headers
	}
}
