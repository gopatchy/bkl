package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gopatchy/bkl"
	"github.com/gopatchy/bkl/pkg/version"
	"github.com/jessevdk/go-flags"
)

type options struct {
	OutputPath   *flags.Filename `short:"o" long:"output" description:"output file path"`
	OutputFormat *string         `short:"f" long:"format" description:"output format" choice:"json" choice:"json-pretty" choice:"jsonl" choice:"toml" choice:"yaml"`
	Version      bool            `short:"v" long:"version" description:"print version and exit"`

	Positional struct {
		InputPath flags.Filename `positional-arg-name:"layerPath" description:"lower layer file path"`
	} `positional-args:"yes"`
}

func main() {
	debug.SetGCPercent(-1)

	opts := &options{}

	fp := flags.NewParser(opts, flags.Default)
	fp.LongDescription = `
bklr generates a document containing just the required fields and their ancestors from the lower layer.

See https://bkl.gopatchy.io/#bklr for detailed documentation.`

	_, err := fp.Parse()
	if err != nil {
		os.Exit(1)
	}

	version.PrintVersion(opts.Version)

	if opts.Positional.InputPath == "" {
		fp.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	fsys := os.DirFS("/")
	enc, err := bkl.Required(fsys, string(opts.Positional.InputPath), "/", "", opts.OutputFormat, (*string)(opts.OutputPath), (*string)(&opts.Positional.InputPath))
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
