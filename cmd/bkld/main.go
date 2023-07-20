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
		BasePath   flags.Filename `positional-arg-name:"basePath" required:"true" description:"base layer file path"`
		TargetPath flags.Filename `positional-arg-name:"targetPath" required:"true" description:"target output file path"`
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
bkld generates the minimal intermediate layer needed to create the target output from the base layer.

See https://bkl.gopatchy.io/#bkld for detailed documentation.`

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

	bs := []*bkl.Parser{}
	numDocs := 0

	for _, path := range []string{string(opts.Positional.BasePath), string(opts.Positional.TargetPath)} {
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
		fh, err = os.OpenFile(string(*opts.OutputPath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
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
