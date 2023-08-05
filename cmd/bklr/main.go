package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/gopatchy/bkl"
	"github.com/jessevdk/go-flags"
)

type options struct {
	OutputPath   *flags.Filename `short:"o" long:"output" description:"output file path"`
	OutputFormat *string         `short:"f" long:"format" description:"output format" choice:"json" choice:"json-pretty" choice:"toml" choice:"yaml"`

	Positional struct {
		InputPath flags.Filename `positional-arg-name:"layerPath" required:"true" description:"lower layer file path"`
	} `positional-args:"yes"`
}

func main() {
	if os.Getenv("BKL_VERSION") != "" {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			fatal(fmt.Errorf("ReadBuildInfo() failed")) //nolint:goerr113
		}

		fmt.Printf("%s", bi)
		os.Exit(0)
	}

	opts := &options{}

	fp := flags.NewParser(opts, flags.Default)
	fp.LongDescription = `
bklr generates a document containing just the required fields and their ancestors from the lower layer.

See https://bkl.gopatchy.io/#bklr for detailed documentation.`

	_, err := fp.Parse()
	if err != nil {
		os.Exit(1)
	}

	realPath, format, err := bkl.FileMatch(string(opts.Positional.InputPath))
	if err != nil {
		fatal(err)
	}

	b := bkl.New()

	err = b.MergeFileLayers(realPath)
	if err != nil {
		fatal(err)
	}

	if opts.OutputPath != nil {
		format = strings.TrimPrefix(filepath.Ext(string(*opts.OutputPath)), ".")
	}

	if opts.OutputFormat != nil {
		format = *opts.OutputFormat
	}

	docs := []any{}

	for i := 0; i < b.NumDocuments(); i++ {
		doc, err := b.Document(i)
		if err != nil {
			fatal(err)
		}

		doc, err = required(doc)
		if err != nil {
			fatal(err)
		}

		docs = append(docs, doc)
	}

	f, err := bkl.GetFormat(format)
	if err != nil {
		fatal(err)
	}

	enc, err := f.MarshalStream(docs)
	if err != nil {
		fatal(err)
	}

	fh := os.Stdout

	if opts.OutputPath != nil {
		fh, err = os.OpenFile(string(*opts.OutputPath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			fatal(err)
		}
	}

	_, err = fh.Write(enc)
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
