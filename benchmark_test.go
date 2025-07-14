package bkl_test

import (
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/gopatchy/bkl"
)

func benchEvaluateTest(testCase *bkl.DocExample) ([]byte, error) {
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

func benchRequiredTest(testCase *bkl.DocExample) ([]byte, error) {
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

func benchIntersectTest(testCase *bkl.DocExample) ([]byte, error) {
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

func benchDiffTest(testCase *bkl.DocExample) ([]byte, error) {
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

func benchCompareTest(testCase *bkl.DocExample) ([]byte, error) {
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
					output, err := benchEvaluateTest(testCase)
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
					output, err := benchRequiredTest(testCase)
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
					output, err := benchIntersectTest(testCase)
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
					output, err := benchDiffTest(testCase)
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
					output, err := benchCompareTest(testCase)
					if err != nil {
						b.Fatalf("Unexpected error: %v", err)
					}
					_ = output
				}
			})
		}
	}
}
