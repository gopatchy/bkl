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

	p, err := bkl.New()
	if err != nil {
		fatal(err)
	}

	// Prepare path from current working directory
	preparedPaths, err := p.PreparePathsFromCwd([]string{string(opts.Positional.InputPath)}, "/")
	if err != nil {
		fatal(err)
	}

	// Use RequiredFile helper which handles loading and validation
	fsys := os.DirFS("/")
	out, err := p.RequiredFile(fsys, preparedPaths[0])
	if err != nil {
		fatal(err)
	}

	// Get format from file if not specified
	format := ""
	if opts.OutputPath != nil {
		format = strings.TrimPrefix(filepath.Ext(string(*opts.OutputPath)), ".")
	}

	if opts.OutputFormat != nil {
		format = *opts.OutputFormat
	}

	if format == "" {
		_, f, err := p.FileMatch(fsys, preparedPaths[0])
		if err != nil {
			fatal(err)
		}
		format = f
	}

	enc, err := p.FormatOutput(out, format)
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
