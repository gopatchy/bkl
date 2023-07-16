package bkl_test

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopatchy/bkl"
	"github.com/samber/lo"
)

func ExampleNew() {
	b := bkl.New()

	fmt.Println(b.NumDocuments())
	// Output:
	// 0
}

func ExampleParser() {
	b := bkl.New()

	// Also parses tests/example1/service.yaml
	err := b.MergeFileLayers("tests/example1/service.test.toml")
	if err != nil {
		panic(err)
	}

	if err = b.OutputToWriter(os.Stdout, "json"); err != nil {
		panic(err)
	}
	// Output:
	// {"addr":"127.0.0.1","name":"myService","port":8081}
}

func ExampleParser_Document() {
	b := bkl.New()

	if err := b.MergeFileLayers("tests/example1/service.test.toml"); err != nil {
		panic(err)
	}

	doc, err := b.Document(0)
	if err != nil {
		panic(err)
	}

	fmt.Println(doc)
	// Output:
	// map[addr:127.0.0.1 name:myService port:8081]
}

func ExampleParser_MergeFile() {
	// Compare to Parser.MergeFileLayers example.

	b := bkl.New()

	// Does *not* parse tests/example1/service.yaml
	err := b.MergeFile("tests/example1/service.test.toml")
	if err != nil {
		panic(err)
	}

	if err = b.OutputToWriter(os.Stdout, "json"); err != nil {
		panic(err)
	}
	// Output:
	// {"port":8081}
}

func ExampleParser_MergeFileLayers() {
	// Compare to Parser.MergeFile example.

	b := bkl.New()

	// Also parses tests/example1/service.yaml
	err := b.MergeFileLayers("tests/example1/service.test.toml")
	if err != nil {
		panic(err)
	}

	if err = b.OutputToWriter(os.Stdout, "json"); err != nil {
		panic(err)
	}
	// Output:
	// {"addr":"127.0.0.1","name":"myService","port":8081}
}

func ExampleParser_MergeParser() {
	b1 := bkl.New()
	b2 := bkl.New()

	if err := b1.MergeFileLayers("tests/tree/a.b.yaml"); err != nil {
		panic(err)
	}

	if err := b2.MergeFileLayers("tests/tree/c.d.yaml"); err != nil {
		panic(err)
	}

	err := b2.MergeParser(b1)
	if err != nil {
		panic(err)
	}

	if err = b2.OutputToWriter(os.Stdout, "json"); err != nil {
		panic(err)
	}
	// Output:
	// {"a":1,"b":2,"c":3,"d":4}
}

func ExampleParser_MergePatch() {
	b := bkl.New()

	err := b.MergePatch(0, map[string]any{"a": 1})
	if err != nil {
		panic(err)
	}

	err = b.MergePatch(0, map[string]any{"b": 2})
	if err != nil {
		panic(err)
	}

	if err = b.OutputToWriter(os.Stdout, "json"); err != nil {
		panic(err)
	}
	// Output:
	// {"a":1,"b":2}
}

func ExampleParser_NumDocuments() {
	b := bkl.New()

	if err := b.MergeFileLayers("tests/example1/service.test.toml"); err != nil {
		panic(err)
	}

	fmt.Println(b.NumDocuments())
	// Output:
	// 1
}

func ExampleParser_Output() {
	b := bkl.New()

	if err := b.MergeFileLayers("tests/output-multi/a.yaml"); err != nil {
		panic(err)
	}

	blob, err := b.Output("yaml")
	if err != nil {
		panic(err)
	}

	os.Stdout.Write(blob)
	// Output:
	// a: 1
	// b: 2
	// ---
	// c: 3
}

func ExampleParser_OutputIndex() {
	b := bkl.New()

	if err := b.MergeFileLayers("tests/stream-add/a.b.yaml"); err != nil {
		panic(err)
	}

	blobs, err := b.OutputIndex(1, "yaml") // second document
	if err != nil {
		panic(err)
	}

	os.Stdout.Write(blobs[0]) // first output of second document
	// Output:
	// c: 3
}

func ExampleParser_OutputToFile() {
	b := bkl.New()

	if err := b.MergeFileLayers("tests/output-multi/a.yaml"); err != nil {
		panic(err)
	}

	f, err := os.CreateTemp("", "example")
	if err != nil {
		panic(err)
	}

	defer os.Remove(f.Name())

	err = b.OutputToFile(f.Name(), "toml")
	if err != nil {
		panic(err)
	}

	blob, err := io.ReadAll(f)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(blob))
	// Output:
	// a = 1
	// b = 2
	// ---
	// c = 3
}

func ExampleParser_OutputToWriter() {
	b := bkl.New()

	if err := b.MergeFileLayers("tests/output-multi/a.yaml"); err != nil {
		panic(err)
	}

	err := b.OutputToWriter(os.Stdout, "yaml")
	if err != nil {
		panic(err)
	}
	// Output:
	// a: 1
	// b: 2
	// ---
	// c: 3
}

func ExampleParser_Outputs() {
	b := bkl.New()

	if err := b.MergeFileLayers("tests/stream-add/a.b.yaml"); err != nil {
		panic(err)
	}

	blobs, err := b.Outputs("yaml")
	if err != nil {
		panic(err)
	}

	os.Stdout.Write(blobs[1]) // second overall output
	// Output:
	// c: 3
}

func ExampleParser_SetDebug() {
	log.Default().SetFlags(0)
	log.Default().SetOutput(os.Stdout)

	b := bkl.New()

	b.SetDebug(true)

	if err := b.MergeFileLayers("tests/example1/service.test.toml"); err != nil {
		panic(err)
	}
	// Output:
	// [tests/example1/service.test.toml] loading
	// [tests/example1/service.yaml] loading
	// [tests/example1/service.yaml] merging
	// [tests/example1/service.test.toml] merging
}

func FuzzParser(f *testing.F) {
	for _, dir := range lo.Must(os.ReadDir("tests")) {
		aPattern := filepath.Join("tests", dir.Name(), "a.????")
		aMatches := lo.Must(filepath.Glob(aPattern))

		if len(aMatches) != 1 {
			continue
		}

		aPath := aMatches[0]

		abPattern := filepath.Join("tests", dir.Name(), "a.b.????")
		abMatches := lo.Must(filepath.Glob(abPattern))

		if len(abMatches) == 1 {
			abPath := abMatches[0]

			f.Add(
				filepath.Base(aPath), lo.Must(os.ReadFile(aPath)),
				filepath.Base(abPath), lo.Must(os.ReadFile(abPath)),
			)
		} else {
			f.Add(
				filepath.Base(aPath), lo.Must(os.ReadFile(aPath)),
				"", []byte{},
			)
		}
	}

	f.Fuzz(func(t *testing.T, filename1 string, content1 []byte, filename2 string, content2 []byte) {
		if strings.HasSuffix(filename1, ".yaml") || strings.HasSuffix(filename2, ".yaml") {
			// TODO: Remove once fixed upstream: https://github.com/go-yaml/yaml/issues/932
			return
		}

		path1, err := writeFile(t, filename1, content1)
		if err != nil {
			t.Log(err)
			return
		}

		path2, err := writeFile(t, filename2, content2)
		if err != nil {
			t.Log(err)
			return
		}

		b := bkl.New()
		_ = b.MergeFile(path1)
		_ = b.MergeFile(path2)
	})
}

func writeFile(t *testing.T, filename string, content []byte) (string, error) {
	filename = filepath.Base(filename)

	if strings.HasPrefix(filename, ".") {
		return "", fmt.Errorf("invalid filename") //nolint:goerr113
	}

	path := filepath.Join(t.TempDir(), filename)

	err := os.WriteFile(path, content, 0600)
	if err != nil {
		return "", err
	}

	return path, nil
}
