package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gopatchy/bkl"
	"github.com/jessevdk/go-flags"
)

type options struct {
	OutputPath   *string `short:"o" long:"output" description:"output file path"`
	OutputFormat *string `short:"f" long:"format" description:"output format"`

	Positional struct {
		BasePath   string `positional-arg-name:"basePath" required:"true" description:"base layer file path"`
		TargetPath string `positional-arg-name:"targetPath" required:"true" description:"target layer file path"`
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

	_, err := fp.Parse()
	if err != nil {
		os.Exit(1)
	}

	format := ""
	if opts.OutputFormat != nil {
		format = *opts.OutputFormat
	}

	bs := []*bkl.Parser{}
	numDocs := 0

	for _, path := range []string{opts.Positional.BasePath, opts.Positional.TargetPath} {
		realPath, f, err := bkl.FileMatch(path)
		if err != nil {
			fatal(err)
		}

		if format == "" {
			format = f
		}

		b := bkl.New()
		bs = append(bs, b)

		err = b.MergeFileLayers(realPath)
		if err != nil {
			fatal(err)
		}

		if b.NumDocuments() > numDocs {
			numDocs = b.NumDocuments()
		}
	}

	docs := []any{}

	for i := 0; i < numDocs; i++ {
		var doc any

		switch {
		case i >= bs[0].NumDocuments():
			// No base doc
			doc, err = bs[1].Document(i)
			if err != nil {
				fatal(err)
			}

		case i >= bs[1].NumDocuments():
			// No target doc
			doc = map[string]any{"$output": false}

		default:
			base, err := bs[0].Document(i)
			if err != nil {
				fatal(err)
			}

			target, err := bs[1].Document(i)
			if err != nil {
				fatal(err)
			}

			doc, err = diff(target, base)
			if err != nil {
				fatal(err)
			}
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
		fh, err = os.OpenFile(*opts.OutputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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
