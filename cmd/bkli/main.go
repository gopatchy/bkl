package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gopatchy/bkl"
	"github.com/jessevdk/go-flags"
)

type options struct {
	OutputPath   *flags.Filename `short:"o" long:"output" description:"output file path"`
	OutputFormat *string         `short:"f" long:"format" description:"output format" choice:"json" choice:"json-pretty" choice:"jsonl" choice:"toml" choice:"yaml"`

	Positional struct {
		InputPaths []flags.Filename `positional-arg-name:"targetPath" required:"2" description:"target output file path"`
	} `positional-args:"yes"`
}

func main() {
	debug.SetGCPercent(-1)
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

	paths := make([]string, len(opts.Positional.InputPaths))
	for i, path := range opts.Positional.InputPaths {
		paths[i] = string(path)
	}

	fsys := os.DirFS("/")
	enc, err := bkl.Intersect(fsys, paths, "/", "", opts.OutputFormat, (*string)(opts.OutputPath), &paths[0])
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
