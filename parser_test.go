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
	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

	docs := b.Documents()
	fmt.Println(docs)
	// Output:
	// []
}

func ExampleParser() {
	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

	// Also parses tests/example1/a.yaml
	err = b.MergeFileLayers("tests/example1/a.b.toml")
	if err != nil {
		panic(err)
	}

	if err = b.OutputToWriter(os.Stdout, "json"); err != nil {
		panic(err)
	}
	// Output:
	// {"addr":"127.0.0.1","name":"myService","port":8081}
}

func ExampleParser_Documents() {
	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

	if err := b.MergeFileLayers("tests/example1/a.b.toml"); err != nil {
		panic(err)
	}

	docs := b.Documents()
	fmt.Println(docs[0].Data)
	// Output:
	// map[addr:127.0.0.1 name:myService port:8081]
}

func ExampleParser_MergeFile() {
	// Compare to Parser.MergeFileLayers example.

	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

	// Does *not* parse tests/example1/a.yaml
	err = b.MergeFile("tests/example1/a.b.toml")
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

	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

	// Also parses tests/example1/a.yaml
	err = b.MergeFileLayers("tests/example1/a.b.toml")
	if err != nil {
		panic(err)
	}

	if err = b.OutputToWriter(os.Stdout, "json"); err != nil {
		panic(err)
	}
	// Output:
	// {"addr":"127.0.0.1","name":"myService","port":8081}
}

func ExampleParser_MergeDocument() {
	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

	doc1 := bkl.NewDocumentWithData("a", map[string]any{"a": 1})
	doc2 := bkl.NewDocumentWithData("b", map[string]any{"b": 2})
	doc2.AddParents(doc1)

	err = b.MergeDocument(doc1)
	if err != nil {
		panic(err)
	}

	err = b.MergeDocument(doc2)
	if err != nil {
		panic(err)
	}

	if err = b.OutputToWriter(os.Stdout, "json"); err != nil {
		panic(err)
	}
	// Output:
	// {"a":1,"b":2}
}

func ExampleParser_Output() {
	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

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

func ExampleParser_Output_literal_dollar() {
	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

	if err := b.MergeFileLayers("tests/literal-dollar/a.yaml"); err != nil {
		panic(err)
	}

	blob, err := b.Output("json")
	if err != nil {
		panic(err)
	}

	os.Stdout.Write(blob)
	// Output:
	// {"listKey":["$instance",{"nested":{"$key":"$value"}}],"mapKey":"$pod"}
}

func ExampleParser_OutputToFile() {
	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

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
	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

	if err := b.MergeFileLayers("tests/output-multi/a.yaml"); err != nil {
		panic(err)
	}

	err = b.OutputToWriter(os.Stdout, "yaml")
	if err != nil {
		panic(err)
	}
	// Output:
	// a: 1
	// b: 2
	// ---
	// c: 3
}

func ExampleParser_SetDebug() {
	log.Default().SetFlags(0)
	log.Default().SetOutput(os.Stdout)

	b, err := bkl.New()
	if err != nil {
		panic(err)
	}

	b.SetDebug(true)

	if err := b.MergeFileLayers("tests/example1/a.b.toml"); err != nil {
		panic(err)
	}
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
		if strings.HasSuffix(filepath.Base(filename1), ".yaml") ||
			strings.HasSuffix(filepath.Base(filename1), ".yml") ||
			strings.HasSuffix(filepath.Base(filename2), ".yaml") ||
			strings.HasSuffix(filepath.Base(filename2), ".yml") {
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

		b, err := bkl.New()
		if err != nil {
			t.Fatal(err)
		}

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

	err := os.WriteFile(path, content, 0o600)
	if err != nil {
		return "", err
	}

	return path, nil
}
