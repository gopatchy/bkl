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
	OutputFormat *string         `short:"f" long:"format" description:"output format" choice:"json" choice:"json-pretty" choice:"toml" choice:"yaml"`
	SkipParent   bool            `short:"P" long:"skip-parent" description:"skip loading parent templates"`
	Verbose      bool            `short:"v" long:"verbose" description:"enable verbose logging"`

	Positional struct {
		InputPaths []flags.Filename `positional-arg-name:"inputPath" required:"1" description:"input file path"`
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
bkl interprets layered configuration files from YAML, JSON, and TOML with additional bkl syntax.

See https://bkl.gopatchy.io/ for detailed documentation.

Related tools:
* bklb
* bkld
* bkli
* bklr`

	_, err := fp.Parse()
	if err != nil {
		os.Exit(1)
	}

	p := bkl.New()

	if opts.Verbose {
		p.SetDebug(true)
	}

	format := ""
	if opts.OutputFormat != nil {
		format = *opts.OutputFormat
	}

	for _, path := range opts.Positional.InputPaths {
		fileP := bkl.New()

		if opts.Verbose {
			fileP.SetDebug(true)
		}

		realPath, f, err := bkl.FileMatch(string(path))
		if err != nil {
			fatal(err)
		}

		if format == "" && opts.OutputPath == nil {
			format = f
		}

		if opts.SkipParent {
			err = fileP.MergeFile(realPath)
		} else {
			err = fileP.MergeFileLayers(realPath)
		}

		if err != nil {
			fatal(err)
		}

		err = p.MergeParser(fileP)
		if err != nil {
			fatal(err)
		}
	}

	if opts.OutputPath == nil {
		err = p.OutputToWriter(os.Stdout, format)
	} else {
		err = p.OutputToFile(string(*opts.OutputPath), format)
	}

	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
