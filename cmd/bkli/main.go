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
		InputPaths []string `positional-arg-name:"inputPath" required:"2" description:"input file path"`
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

	var docs []any

	for p, path := range opts.Positional.InputPaths {
		realPath, f, err := bkl.FileMatch(path)
		if err != nil {
			continue
		}

		format = f

		b := bkl.New()

		err = b.MergeFileLayers(realPath)
		if err != nil {
			fatal(err)
		}

		for i := 0; i < b.NumDocuments(); i++ {
			if i == len(docs) {
				docs = append(docs, nil)
			}

			doc, err := b.Document(i)
			if err != nil {
				fatal(err)
			}

			if p == 0 {
				docs[i] = doc
				continue
			}

			docs[i], err = intersect(docs[i], doc)
			if err != nil {
				fatal(err)
			}
		}
	}

	b := bkl.New()
	b.SetRequired(false)

	for i, doc := range docs {
		err = b.MergePatch(i, doc)
		if err != nil {
			fatal(err)
		}
	}

	if opts.OutputPath == nil {
		err = b.OutputToWriter(os.Stdout, format)
	} else {
		err = b.OutputToFile(*opts.OutputPath, format)
	}

	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
