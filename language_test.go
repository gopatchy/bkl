package bkl

import (
	"bytes"
	"flag"
	"os"
	"testing"
	"testing/fstest"

	"github.com/pelletier/go-toml/v2"
)

type TestCase struct {
	Eval     string
	Format   string
	Expected string
	Files    map[string]string
}

type TestSuite map[string]TestCase

var singleTest = flag.String("test.single", "", "Run only the specified test from tests.toml")

func TestLanguage(t *testing.T) {
	// Load the test suite
	data, err := os.ReadFile("tests.toml")
	if err != nil {
		t.Fatalf("Failed to read tests.toml: %v", err)
	}

	var suite TestSuite
	err = toml.Unmarshal(data, &suite)
	if err != nil {
		t.Fatalf("Failed to parse tests.toml: %v", err)
	}

	for testName, testCase := range suite {
		// Skip tests that don't match the single test filter
		if *singleTest != "" && testName != *singleTest {
			continue
		}
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			// Create in-memory filesystem
			fsys := fstest.MapFS{}

			// Add all files
			for filename, content := range testCase.Files {
				fsys[filename] = &fstest.MapFile{
					Data: []byte(content),
				}
			}

			// Create parser with in-memory FS
			p, err := NewWithFS(fsys, "/")
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			// Load and process the file
			err = p.MergeFileLayers(testCase.Eval)
			if err != nil {
				t.Fatalf("Failed to merge file layers: %v", err)
			}

			// Get output
			output, err := p.Output(testCase.Format)
			if err != nil {
				t.Fatalf("Failed to get output: %v", err)
			}

			// Compare output
			if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(testCase.Expected))) {
				t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", testCase.Expected, output)
			}
		})
	}
}
