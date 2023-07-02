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

	for _, path := range opts.Positional.InputPaths {
		err := p.MergeFileLayers(path)
		if err != nil {
			fmt.Printf("%s\n", err)
			os.Exit(1)
		}
	}
}
