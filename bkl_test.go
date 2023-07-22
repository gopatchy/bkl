package bkl_test

import (
	"os"

	"github.com/gopatchy/bkl"
)

func Example() {
	// import "github.com/gopatchy/bkl"

	b := bkl.New()

	err := b.MergeFileLayers("tests/example1/a.b.toml")
	if err != nil {
		panic(err)
	}

	err = b.OutputToWriter(os.Stdout, "json")
	if err != nil {
		panic(err)
	}
	// Output:
	// {"addr":"127.0.0.1","name":"myService","port":8081}
}
