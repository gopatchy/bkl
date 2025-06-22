package bkl_test

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gopatchy/bkl"
	"github.com/pelletier/go-toml/v2"
)

type TestCase struct {
	Description string
	Eval        []string
	Format      string
	Expected    string
	Files       map[string]string
	ErrorMerge  string // Expected error from MergeFileLayers
	ErrorOutput string // Expected error from Output
}

type TestSuite map[string]TestCase

var testFilter = flag.String("test.filter", "", "Run only specified tests from tests.toml (comma-separated list)")

func TestLanguage(t *testing.T) {
	data, err := os.ReadFile("tests.toml")
	if err != nil {
		t.Fatalf("Failed to read tests.toml: %v", err)
	}

	var suite TestSuite
	err = toml.Unmarshal(data, &suite)
	if err != nil {
		t.Fatalf("Failed to parse tests.toml: %v", err)
	}

	// Parse filter list
	filterTests := map[string]bool{}
	if *testFilter != "" {
		for _, name := range strings.Split(*testFilter, ",") {
			name = strings.TrimSpace(name)
			if name != "" {
				if _, ok := suite[name]; !ok {
					t.Fatalf("Test %q not found in tests.toml", name)
				}
				filterTests[name] = true
			}
		}
	}

	for testName, testCase := range suite {
		if len(filterTests) > 0 && !filterTests[testName] {
			continue
		}

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			fsys := fstest.MapFS{}

			for filename, content := range testCase.Files {
				fsys[filename] = &fstest.MapFile{
					Data: []byte(content),
				}
			}

			p, err := bkl.NewWithFS(fsys, "/")
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			var mergeErr error
			for _, evalFile := range testCase.Eval {
				err = p.MergeFileLayers(evalFile)
				if err != nil {
					mergeErr = err
					break
				}
			}

			if testCase.ErrorMerge != "" {
				if mergeErr == nil {
					t.Fatalf("Expected merge error containing %q, but got no error", testCase.ErrorMerge)
				}
				if !strings.Contains(mergeErr.Error(), testCase.ErrorMerge) {
					t.Fatalf("Expected merge error containing %q, but got: %v", testCase.ErrorMerge, mergeErr)
				}
				return
			}
			if mergeErr != nil {
				t.Fatalf("Unexpected merge error: %v", mergeErr)
			}

			output, err := p.Output(testCase.Format)

			if testCase.ErrorOutput != "" {
				if err == nil {
					t.Fatalf("Expected output error containing %q, but got no error", testCase.ErrorOutput)
				}
				if !strings.Contains(err.Error(), testCase.ErrorOutput) {
					t.Fatalf("Expected output error containing %q, but got: %v", testCase.ErrorOutput, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected output error: %v", err)
			}

			if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(testCase.Expected))) {
				t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", testCase.Expected, output)
			}
		})
	}
}
