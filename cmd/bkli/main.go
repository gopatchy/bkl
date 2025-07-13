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
	Selectors    []string        `short:"s" long:"selector" description:"selector expression to match documents (e.g. 'metadata.name'), can be specified multiple times"`
	Version      bool            `short:"v" long:"version" description:"print version and exit"`

	Positional struct {
		InputPaths []flags.Filename `positional-arg-name:"targetPath" description:"target output file path"`
	} `positional-args:"yes"`
}

func main() {
	debug.SetGCPercent(-1)

	opts := &options{}

	fp := flags.NewParser(opts, flags.Default)
	fp.LongDescription = `
bkli generates the maximal base layer that the specified targets have in common.

See https://bkl.gopatchy.io/#bkli for detailed documentation.`

	_, err := fp.Parse()
	if err != nil {
		os.Exit(1)
	}

	version.PrintVersion(opts.Version)

	if len(opts.Positional.InputPaths) < 2 {
		fp.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	paths := make([]string, len(opts.Positional.InputPaths))
	for i, path := range opts.Positional.InputPaths {
		paths[i] = string(path)
	}

	fsys := os.DirFS("/")
	enc, err := bkl.Intersect(fsys, paths, "/", "", opts.Selectors, opts.OutputFormat, (*string)(opts.OutputPath), &paths[0])
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
