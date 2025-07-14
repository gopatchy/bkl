package bkl_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gopatchy/bkl"
)

var exportRegex = regexp.MustCompile(`#\s*export\s+([A-Z_]+)=(.*)`)

func runEvaluateTest(testCase *bkl.DocExample) ([]byte, error) {
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

func runRequiredTest(testCase *bkl.DocExample) ([]byte, error) {
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

func runIntersectTest(testCase *bkl.DocExample) ([]byte, error) {
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

func runDiffTest(testCase *bkl.DocExample) ([]byte, error) {
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

func runCompareTest(testCase *bkl.DocExample) ([]byte, error) {
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

func runTestBKLEvaluate(t *testing.T, testCase *bkl.DocExample) {
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

func runTestBKLRequired(t *testing.T, testCase *bkl.DocExample) {
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

func runTestBKLIntersect(t *testing.T, testCase *bkl.DocExample) {
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

func runTestBKLDiff(t *testing.T, testCase *bkl.DocExample) {
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

func runTestBKLCompare(t *testing.T, testCase *bkl.DocExample) {
	output, err := runCompareTest(testCase)
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

	for testName, testCase := range tests {
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

func prepareEvaluateExample(example *bkl.DocExample, acceptableLanguages []string) bool {
	if !validateLanguagePointers(example.Evaluate.Inputs, acceptableLanguages) {
		return false
	}

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
		return false
	}

	example.Evaluate = &bkl.DocEvaluate{
		Inputs: inputs,
		Result: example.Evaluate.Result,
		Env:    env,
	}

	return true
}

func prepareConvertExample(example *bkl.DocExample, acceptableLanguages []string) bool {
	if !validateLanguage([]bkl.DocLayer{example.Convert.To}, acceptableLanguages) {
		return false
	}

	lang := example.Convert.To.Languages[0][1].(string)
	filename := fmt.Sprintf("file.%s", lang)

	env := map[string]string{}
	for k, v := range extractEnvVars(example.Convert.To.Code) {
		env[k] = v
	}

	if len(example.Convert.From.Languages) != 1 {
		return false
	}

	// Convert to Evaluate for testing
	example.Evaluate = &bkl.DocEvaluate{
		Inputs: []*bkl.DocLayer{{
			Filename:  filename,
			Code:      example.Convert.To.Code,
			Languages: example.Convert.To.Languages,
		}},
		Result: example.Convert.From,
		Env:    env,
	}
	example.Convert = nil

	return true
}

func prepareFixitExample(example *bkl.DocExample, acceptableLanguages []string) bool {
	goodLayer := example.Fixit.Good

	if len(goodLayer.Languages) != 1 {
		return false
	}

	lang := goodLayer.Languages[0][1].(string)
	if !slices.Contains(acceptableLanguages, lang) {
		return false
	}

	filename := "good." + lang

	// Convert to Evaluate for testing
	example.Evaluate = &bkl.DocEvaluate{
		Inputs: []*bkl.DocLayer{{
			Filename:  filename,
			Code:      goodLayer.Code,
			Languages: goodLayer.Languages,
		}},
		Result: goodLayer,
		Env:    extractEnvVars(goodLayer.Code),
	}
	// Keep Fixit for special handling in runDocumentationTest

	return true
}

func prepareDiffExample(example *bkl.DocExample, acceptableLanguages []string) bool {
	if !validateLanguage([]bkl.DocLayer{example.Diff.Base, example.Diff.Target}, acceptableLanguages) {
		return false
	}

	baseLang := example.Diff.Base.Languages[0][1].(string)
	targetLang := example.Diff.Target.Languages[0][1].(string)

	base := example.Diff.Base
	base.Filename = "file0." + baseLang

	target := example.Diff.Target
	target.Filename = "file1." + targetLang

	if len(example.Diff.Result.Languages) != 1 {
		return false
	}

	example.Diff = &bkl.DocDiff{
		Base:   base,
		Target: target,
		Result: example.Diff.Result,
	}

	return true
}

func prepareIntersectExample(example *bkl.DocExample, acceptableLanguages []string) bool {
	if !validateLanguagePointers(example.Intersect.Inputs, acceptableLanguages) {
		return false
	}

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
		return false
	}

	example.Intersect = &bkl.DocIntersect{
		Inputs: inputs,
		Result: example.Intersect.Result,
	}

	return true
}

func runDocumentationTest(t *testing.T, testCase *bkl.DocExample, example *bkl.DocExample) {
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

			// Skip Compare examples for now
			if example.Compare != nil {
				continue
			}

			// Prepare the example for testing
			testCase := &bkl.DocExample{}
			*testCase = *example // Copy the example

			// Process based on which operation is present
			ok := false
			switch {
			case testCase.Evaluate != nil:
				ok = prepareEvaluateExample(testCase, acceptableLanguages)
			case testCase.Convert != nil:
				ok = prepareConvertExample(testCase, acceptableLanguages)
			case testCase.Fixit != nil:
				ok = prepareFixitExample(testCase, acceptableLanguages)
			case testCase.Diff != nil:
				ok = prepareDiffExample(testCase, acceptableLanguages)
			case testCase.Intersect != nil:
				ok = prepareIntersectExample(testCase, acceptableLanguages)
			default:
				continue
			}

			if !ok {
				continue
			}

			testCase.Description = fmt.Sprintf("Doc example from %s", section.Title)

			if section.ID == "bklr" && testCase.Evaluate != nil {
				testCase.Required = testCase.Evaluate
				testCase.Evaluate = nil
			}

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
