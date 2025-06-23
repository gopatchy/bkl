package bkl_test

import (
	"bytes"
	"flag"
	"io/fs"
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
	Error       string            // Expected error from evaluation
	SkipParent  bool              // Skip loading parent templates
	RootPath    string            // Root path for restricting file access
	Env         map[string]string // Environment variables for the test
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

			rootPath := testCase.RootPath
			if rootPath == "" {
				rootPath = "/"
			}

			// Create a filesystem view rooted at the rootPath
			var testFS fs.FS = fsys
			if rootPath != "/" {
				var err error
				testFS, err = fs.Sub(fsys, rootPath)
				if err != nil {
					t.Fatalf("Failed to create sub filesystem at %s: %v", rootPath, err)
				}
			}

			p, err := bkl.New()
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			output, err := p.Evaluate(testFS, testCase.Eval, testCase.SkipParent, testCase.Format, rootPath, "/", testCase.Env)

			if testCase.Error != "" {
				if err == nil {
					t.Fatalf("Expected error containing %q, but got no error", testCase.Error)
				}
				if !strings.Contains(err.Error(), testCase.Error) {
					t.Fatalf("Expected error containing %q, but got: %v", testCase.Error, err)
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
