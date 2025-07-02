package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"runtime/pprof"

	"github.com/gopatchy/bkl"
	"github.com/gopatchy/bkl/pkg/log"
	"github.com/gopatchy/bkl/pkg/version"
	"github.com/jessevdk/go-flags"
)

type options struct {
	OutputPath   *flags.Filename `short:"o" long:"output" description:"output file path"`
	OutputFormat *string         `short:"f" long:"format" description:"output format" choice:"json" choice:"json-pretty" choice:"jsonl" choice:"toml" choice:"yaml"`
	RootPath     string          `short:"r" long:"root-path" description:"restrict file access to this root directory" default:"/"`
	SortPath     string          `short:"s" long:"sort" description:"sort output documents by path (e.g. 'metadata.name')"`
	Verbose      bool            `short:"v" long:"verbose" description:"enable verbose logging"`
	Version      bool            `short:"V" long:"version" description:"print version and exit"`

	CPUProfile *string `short:"c" long:"cpu-profile" description:"write CPU profile to file"`

	Positional struct {
		InputPaths []flags.Filename `positional-arg-name:"inputPath" required:"0" description:"input file path"`
	} `positional-args:"yes"`
}

func main() {
	debug.SetGCPercent(-1)

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

	if opts.CPUProfile != nil {
		fh, err := os.Create(*opts.CPUProfile)
		if err != nil {
			fatal(err)
		}

		pprof.StartCPUProfile(fh)
		defer pprof.StopCPUProfile()
	}

	version.PrintVersion(opts.Version)

	if len(opts.Positional.InputPaths) == 0 {
		fp.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	if opts.Verbose {
		log.Debug = true
	}

	files := make([]string, len(opts.Positional.InputPaths))
	for i, path := range opts.Positional.InputPaths {
		files[i] = string(path)
	}

	root, err := os.OpenRoot(opts.RootPath)
	if err != nil {
		fatal(err)
	}
	defer root.Close()

	output, err := bkl.Evaluate(root.FS(), files, opts.RootPath, "", nil, opts.OutputFormat, opts.SortPath, (*string)(opts.OutputPath), &files[0])
	if err != nil {
		fatal(err)
	}

	if opts.OutputPath == nil {
		_, err = os.Stdout.Write(output)
	} else {
		err = os.WriteFile(string(*opts.OutputPath), output, 0o644)
	}

	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
