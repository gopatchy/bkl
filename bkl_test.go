package bkl_test

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gopatchy/bkl"
)

func validateError(t *testing.T, err error, expectedErrors []string) {
	if len(expectedErrors) > 0 {
		if err == nil {
			t.Fatalf("Expected error containing one of %v, but got no error", expectedErrors)
		}

		errorFound := false
		for _, expectedError := range expectedErrors {
			if strings.Contains(err.Error(), expectedError) {
				errorFound = true
				break
			}
		}

		if !errorFound {
			t.Fatalf("Expected error containing one of %v, but got: %v", expectedErrors, err)
		}
	} else if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func validateOutput(t *testing.T, output []byte, expected string) {
	if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(expected))) {
		t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", expected, output)
	}
}

func runEvaluateTest(t *testing.T, testCase *bkl.DocExample) {
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

	var evalFiles []string
	if len(allFiles) > 0 {
		evalFiles = []string{allFiles[len(allFiles)-1]}
	}

	var testFS fs.FS = fsys
	if rootPath != "/" {
		var err error
		testFS, err = fs.Sub(fsys, rootPath)
		if err != nil {
			t.Fatalf("Failed to create sub filesystem: %v", err)
		}
	}

	format := getFormat(testCase.Evaluate.Result.Languages)
	var firstFile *string
	if len(evalFiles) > 0 {
		firstFile = &evalFiles[0]
	}

	output, err := bkl.Evaluate(testFS, evalFiles, rootPath, rootPath, testCase.Evaluate.Env, format, testCase.Evaluate.Sort, firstFile)

	validateError(t, err, testCase.Evaluate.Errors)
	if err == nil {
		validateOutput(t, output, testCase.Evaluate.Result.Code)
	}
}

func runRequiredTest(t *testing.T, testCase *bkl.DocExample) {
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
		t.Fatalf("Required tests require exactly 1 eval file, got %d", len(evalFiles))
	}

	var testFS fs.FS = fsys
	if rootPath != "/" {
		var err error
		testFS, err = fs.Sub(fsys, rootPath)
		if err != nil {
			t.Fatalf("Failed to create sub filesystem: %v", err)
		}
	}

	format := getFormat(testCase.Required.Result.Languages)
	firstFile := &evalFiles[0]

	output, err := bkl.Required(testFS, evalFiles[0], rootPath, rootPath, format, firstFile)

	validateError(t, err, testCase.Required.Errors)
	if err == nil {
		validateOutput(t, output, testCase.Required.Result.Code)
	}
}

func runIntersectTest(t *testing.T, testCase *bkl.DocExample) {
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
		t.Fatalf("Intersect tests require at least 2 eval files, got %d", len(evalFiles))
	}

	format := getFormat(testCase.Intersect.Result.Languages)
	var firstFile *string
	if len(evalFiles) > 0 {
		firstFile = &evalFiles[0]
	}

	output, err := bkl.Intersect(fsys, evalFiles, rootPath, rootPath, testCase.Intersect.Selector, format, firstFile)

	validateError(t, err, testCase.Intersect.Errors)
	if err == nil {
		validateOutput(t, output, testCase.Intersect.Result.Code)
	}
}

func runDiffTest(t *testing.T, testCase *bkl.DocExample) {
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

	output, err := bkl.Diff(fsys, testCase.Diff.Base.Filename, testCase.Diff.Target.Filename, rootPath, rootPath, testCase.Diff.Selector, format, firstFile)

	validateError(t, err, testCase.Diff.Errors)
	if err == nil {
		validateOutput(t, output, testCase.Diff.Result.Code)
	}
}

func runCompareTest(t *testing.T, testCase *bkl.DocExample) {
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
		t.Fatalf("Unexpected error: %v", err)
	}

	validateOutput(t, []byte(result.Diff), testCase.Compare.Result.Code)
}

func TestBKL(t *testing.T) {
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
				runEvaluateTest(t, testCase)
			case testCase.Required != nil:
				runRequiredTest(t, testCase)
			case testCase.Intersect != nil:
				runIntersectTest(t, testCase)
			case testCase.Diff != nil:
				runDiffTest(t, testCase)
			case testCase.Compare != nil:
				runCompareTest(t, testCase)
			}
		})
	}
}
