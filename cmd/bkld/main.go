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
		BasePath   flags.Filename `positional-arg-name:"basePath" required:"true" description:"base layer file path"`
		TargetPath flags.Filename `positional-arg-name:"targetPath" required:"true" description:"target output file path"`
	} `positional-args:"yes"`
}

func main() {
	debug.SetGCPercent(-1)
	if os.Getenv("BKL_VERSION") != "" {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			fatal(fmt.Errorf("ReadBuildInfo() failed"))
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

	fsys := os.DirFS("/")
	enc, err := bkl.Diff(fsys, string(opts.Positional.BasePath), string(opts.Positional.TargetPath), "/", "", opts.OutputFormat, (*string)(opts.OutputPath), (*string)(&opts.Positional.BasePath))
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
	_, _ = fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
