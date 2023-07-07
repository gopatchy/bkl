package main

import (
	"fmt"
	"os"

	"github.com/gopatchy/bkl"
	"github.com/jessevdk/go-flags"
)

type options struct {
	OutputPath   *string `short:"o" long:"output" description:"output file path"`
	OutputFormat *string `short:"f" long:"format" description:"output format"`
	SkipParent   bool    `short:"P" long:"skip-parent" description:"skip loading parent templates"`
	Verbose      bool    `short:"v" long:"verbose" description:"enable verbose logging"`

	Positional struct {
		InputPaths []string `positional-arg-name:"inputPath" required:"1" description:"input file path"`
	} `positional-args:"yes"`
}

func main() {
	opts := &options{}

	fp := flags.NewParser(opts, flags.Default)

	_, err := fp.Parse()
	if flags.WroteHelp(err) {
		os.Exit(1)
	}

	p := bkl.New()

	if opts.Verbose {
		p.SetDebug(true)
	}

	for _, path := range opts.Positional.InputPaths {
		fileP := bkl.New()

		if opts.Verbose {
			fileP.SetDebug(true)
		}

		if opts.SkipParent {
			err = fileP.MergeFile(path)
		} else {
			err = fileP.MergeFileLayers(path)
		}
		if err != nil {
			fatal(err)
		}

		err = p.MergeParser(fileP)
		if err != nil {
			fatal(err)
		}
	}

	format := ""
	if opts.OutputFormat != nil {
		format = *opts.OutputFormat
	}

	if opts.OutputPath == nil {
		err = p.OutputToWriter(os.Stdout, format)
	} else {
		err = p.OutputToFile(*opts.OutputPath, format)
	}

	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
