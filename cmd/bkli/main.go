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
	OutputFormat *string         `short:"f" long:"format" description:"output format"  choice:"json" choice:"json-pretty" choice:"toml" choice:"yaml"`

	Positional struct {
		InputPaths []flags.Filename `positional-arg-name:"targetPath" required:"2" description:"target output file path"`
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
bkli generates the maximal base layer that the specified targets have in common.

See https://bkl.gopatchy.io/#bkli for detailed documentation.`

	_, err := fp.Parse()
	if err != nil {
		os.Exit(1)
	}

	format := ""

	if opts.OutputFormat != nil {
		format = *opts.OutputFormat
	}

	if format == "" && opts.OutputPath != nil {
		format = strings.TrimPrefix(filepath.Ext(string(*opts.OutputPath)), ".")
	}

	var doc any

	for p, path := range opts.Positional.InputPaths {
		realPath, f, err := bkl.FileMatch(string(path))
		if err != nil {
			fatal(err)
		}

		if format == "" {
			format = f
		}

		b := bkl.New()

		err = b.MergeFileLayers(realPath)
		if err != nil {
			fatal(err)
		}

		docs, err := b.Documents()
		if err != nil {
			fatal(err)
		}

		if len(docs) != 1 {
			fatal(fmt.Errorf("bklr operates on exactly 1 source document"))
		}

		if p == 0 {
			doc = docs[0]
			continue
		}

		doc, err = intersect(docs[0], doc)
		if err != nil {
			fatal(err)
		}
	}

	f, err := bkl.GetFormat(format)
	if err != nil {
		fatal(err)
	}

	enc, err := f.MarshalStream([]any{doc})
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
