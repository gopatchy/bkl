package bkl_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gopatchy/bkl"
)

func getFormat(languages [][]any) *string {
	if len(languages) > 0 && len(languages[0]) > 1 {
		if format, ok := languages[0][1].(string); ok {
			return &format
		}
	}
	return nil
}

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

func validateOutput(t *testing.T, output []byte, expected string, removeInitialLines int) {
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

func runEvaluateTest(t *testing.T, evaluate *bkl.DocEvaluate) {
	fsys := fstest.MapFS{}
	rootPath := evaluate.Root
	if rootPath == "" {
		rootPath = "/"
	}

	var allFiles []string
	for _, input := range evaluate.Inputs {
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

	format := getFormat(evaluate.Result.Languages)
	var firstFile *string
	if len(evalFiles) > 0 {
		firstFile = &evalFiles[0]
	}

	output, err := bkl.Evaluate(testFS, evalFiles, rootPath, rootPath, evaluate.Env, format, evaluate.Sort, firstFile)

	validateError(t, err, evaluate.Errors)
	if err == nil {
		validateOutput(t, output, evaluate.Result.Code, 0)
	}
}

func runRequiredTest(t *testing.T, required *bkl.DocRequired) {
	fsys := fstest.MapFS{}
	rootPath := required.Root
	if rootPath == "" {
		rootPath = "/"
	}

	var evalFiles []string
	for _, input := range required.Inputs {
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

	format := getFormat(required.Result.Languages)
	firstFile := &evalFiles[0]

	output, err := bkl.Required(testFS, evalFiles[0], rootPath, rootPath, format, firstFile)

	validateError(t, err, required.Errors)
	if err == nil {
		validateOutput(t, output, required.Result.Code, 0)
	}
}

func runIntersectTest(t *testing.T, intersect *bkl.DocIntersect) {
	fsys := fstest.MapFS{}
	rootPath := "/"

	var evalFiles []string
	for _, input := range intersect.Inputs {
		fsys[input.Filename] = &fstest.MapFile{
			Data: []byte(input.Code),
		}
		evalFiles = append(evalFiles, input.Filename)
	}

	if len(evalFiles) < 2 {
		t.Fatalf("Intersect tests require at least 2 eval files, got %d", len(evalFiles))
	}

	format := getFormat(intersect.Result.Languages)
	var firstFile *string
	if len(evalFiles) > 0 {
		firstFile = &evalFiles[0]
	}

	output, err := bkl.Intersect(fsys, evalFiles, rootPath, rootPath, intersect.Selector, format, firstFile)

	validateError(t, err, intersect.Errors)
	if err == nil {
		validateOutput(t, output, intersect.Result.Code, 0)
	}
}

func runDiffTest(t *testing.T, diff *bkl.DocDiff) {
	fsys := fstest.MapFS{}
	rootPath := "/"

	fsys[diff.Base.Filename] = &fstest.MapFile{
		Data: []byte(diff.Base.Code),
	}
	fsys[diff.Target.Filename] = &fstest.MapFile{
		Data: []byte(diff.Target.Code),
	}

	format := getFormat(diff.Result.Languages)
	firstFile := &diff.Base.Filename

	output, err := bkl.Diff(fsys, diff.Base.Filename, diff.Target.Filename, rootPath, rootPath, diff.Selector, format, firstFile)

	validateError(t, err, diff.Errors)
	if err == nil {
		validateOutput(t, output, diff.Result.Code, 0)
	}
}

func runCompareTest(t *testing.T, compare *bkl.DocCompare) {
	fsys := fstest.MapFS{}
	rootPath := "/"

	fsys[compare.Left.Filename] = &fstest.MapFile{
		Data: []byte(compare.Left.Code),
	}
	fsys[compare.Right.Filename] = &fstest.MapFile{
		Data: []byte(compare.Right.Code),
	}

	format := getFormat(compare.Result.Languages)

	result, err := bkl.Compare(fsys, compare.Left.Filename, compare.Right.Filename, rootPath, rootPath, compare.Env, format, compare.Sort)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateOutput(t, []byte(result.Diff), compare.Result.Code, 2)
}

func runConvertTest(t *testing.T, convert *bkl.DocConvert) {
	fsys := fstest.MapFS{}
	rootPath := "/"

	if len(convert.To.Languages) != 1 {
		t.Fatalf("Convert test requires exactly one language in To layer")
	}
	lang := convert.To.Languages[0][1].(string)
	filename := fmt.Sprintf("file.%s", lang)

	fsys[filename] = &fstest.MapFile{
		Data: []byte(convert.To.Code),
	}

	format := getFormat(convert.From.Languages)
	firstFile := &filename

	output, err := bkl.Evaluate(fsys, []string{filename}, rootPath, rootPath, nil, format, nil, firstFile)

	validateError(t, err, nil)
	if err == nil {
		validateOutput(t, output, convert.From.Code, 0)
	}
}

func runFixitTest(t *testing.T, fixit *bkl.DocFixit) {
	fsys := fstest.MapFS{}
	rootPath := "/"

	if len(fixit.Good.Languages) != 1 {
		t.Fatalf("Fixit test requires exactly one language in Good layer")
	}
	lang := fixit.Good.Languages[0][1].(string)
	filename := fmt.Sprintf("good.%s", lang)

	fsys[filename] = &fstest.MapFile{
		Data: []byte(fixit.Good.Code),
	}

	format := getFormat(fixit.Good.Languages)
	firstFile := &filename

	output, err := bkl.Evaluate(fsys, []string{filename}, rootPath, rootPath, nil, format, nil, firstFile)

	validateError(t, err, nil)
	if err == nil {
		if fixit.Original.Code != "" {
			validateOutput(t, output, fixit.Original.Code, 0)
		} else {
			validateOutput(t, output, fixit.Good.Code, 0)
		}
	}
}

func RunTestLoop(t *testing.T, tests map[string]*bkl.DocExample) {
	for testName, testCase := range tests {
		if testCase.Benchmark {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			switch {
			case testCase.Evaluate != nil:
				runEvaluateTest(t, testCase.Evaluate)
			case testCase.Required != nil:
				runRequiredTest(t, testCase.Required)
			case testCase.Intersect != nil:
				runIntersectTest(t, testCase.Intersect)
			case testCase.Diff != nil:
				runDiffTest(t, testCase.Diff)
			case testCase.Compare != nil:
				runCompareTest(t, testCase.Compare)
			case testCase.Convert != nil:
				runConvertTest(t, testCase.Convert)
			case testCase.Fixit != nil:
				runFixitTest(t, testCase.Fixit)
			}
		})
	}
}

func TestBKL(t *testing.T) {
	t.Parallel()

	tests, err := bkl.GetAllTests()
	if err != nil {
		t.Fatalf("Failed to get all tests: %v", err)
	}

	RunTestLoop(t, tests)
}
