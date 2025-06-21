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
	Description string
	Eval        string
	Format      string
	Expected    string
	Files       map[string]string
}

type TestSuite map[string]TestCase

var singleTest = flag.String("test.single", "", "Run only the specified test from tests.toml")

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

	for testName, testCase := range suite {
		if *singleTest != "" && testName != *singleTest {
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

			p, err := NewWithFS(fsys, "/")
			if err != nil {
				t.Fatalf("Failed to create parser: %v", err)
			}

			err = p.MergeFileLayers(testCase.Eval)
			if err != nil {
				t.Fatalf("Failed to merge file layers: %v", err)
			}

			output, err := p.Output(testCase.Format)
			if err != nil {
				t.Fatalf("Failed to get output: %v", err)
			}

			if !bytes.Equal(bytes.TrimSpace(output), bytes.TrimSpace([]byte(testCase.Expected))) {
				t.Errorf("Output mismatch\nExpected:\n%s\nGot:\n%s", testCase.Expected, output)
			}
		})
	}
}
