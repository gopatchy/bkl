package bkl_test

import (
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

	example.Evaluate = &bkl.DocEvaluate{
		Inputs: []*bkl.DocLayer{{
			Filename:  filename,
			Code:      goodLayer.Code,
			Languages: goodLayer.Languages,
		}},
		Result: goodLayer,
		Env:    extractEnvVars(goodLayer.Code),
	}
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

func runEvaluateTestForDoc(testCase *bkl.DocExample) ([]byte, error) {
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

func runDocumentationTest(t *testing.T, testCase *bkl.DocExample, example *bkl.DocExample) {
	var output []byte
	var err error

	switch {
	case testCase.Evaluate != nil:
		output, err = runEvaluateTestForDoc(testCase)
	case testCase.Required != nil:
		runRequiredTest(t, testCase)
		return
	case testCase.Intersect != nil:
		runIntersectTest(t, testCase)
		return
	case testCase.Diff != nil:
		runDiffTest(t, testCase)
		return
	case testCase.Compare != nil:
		runCompareTest(t, testCase)
		return
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

	if testCase.Evaluate != nil {
		validateError(t, err, testCase.Evaluate.Errors)
		if err == nil {
			validateOutput(t, output, testCase.Evaluate.Result.Code)
		}
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

			if example.Compare != nil {
				continue
			}

			testCase := &bkl.DocExample{}
			*testCase = *example

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
