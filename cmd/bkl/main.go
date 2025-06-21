package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"strings"

	"github.com/gopatchy/bkl"
	"github.com/jessevdk/go-flags"
	"golang.org/x/exp/constraints"
)

type options struct {
	OutputPath   *flags.Filename `short:"o" long:"output" description:"output file path"`
	OutputFormat *string         `short:"f" long:"format" description:"output format" choice:"json" choice:"json-pretty" choice:"toml" choice:"yaml"`
	RootPath     string          `short:"r" long:"root-path" description:"restrict file access to this root directory" default:"/"`
	SkipParent   bool            `short:"P" long:"skip-parent" description:"skip loading parent templates"`
	Verbose      bool            `short:"v" long:"verbose" description:"enable verbose logging"`
	Version      bool            `short:"V" long:"version" description:"print version and exit"`

	CPUProfile *string `short:"c" long:"cpu-profile" description:"write CPU profile to file"`

	Positional struct {
		InputPaths []flags.Filename `positional-arg-name:"inputPath" required:"0" description:"input file path"`
	} `positional-args:"yes"`
}

func main() {
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

	if opts.Version || os.Getenv("BKL_VERSION") != "" {
		version()
	}

	if len(opts.Positional.InputPaths) == 0 {
		fp.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	wd, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

	absRootPath, err := filepath.Abs(opts.RootPath)
	if err != nil {
		fatal(err)
	}

	relWd, err := filepath.Rel(absRootPath, wd)
	if err != nil {
		fatal(err)
	}

	p, err := bkl.NewWithPath(absRootPath, relWd)
	if err != nil {
		fatal(err)
	}

	if opts.Verbose {
		p.SetDebug(true)
	}

	format := ""
	if opts.OutputFormat != nil {
		format = *opts.OutputFormat
	}

	for _, path := range opts.Positional.InputPaths {
		// Convert user-provided path to be relative to root path
		absPath, err := filepath.Abs(string(path))
		if err != nil {
			fatal(err)
		}

		relPath, err := filepath.Rel(absRootPath, absPath)
		if err != nil {
			fatal(err)
		}

		// Add leading "/" to make it absolute within the FS
		relPath = "/" + relPath

		realPath, f, err := p.FileMatch(relPath)
		if err != nil {
			fatal(err)
		}

		if format == "" && opts.OutputPath == nil {
			format = f
		}

		if opts.SkipParent {
			err = p.MergeFile(realPath)
		} else {
			err = p.MergeFileLayers(realPath)
		}

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

func version() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		fatal(fmt.Errorf("ReadBuildInfo() failed")) //nolint:goerr113
	}

	ver, src := versionFromBuildInfo(bi)
	fmt.Printf("%s (%s)\n", ver, src)

	fmt.Printf("%s", bi)
	os.Exit(0)
}

func versionFromBuildInfo(bi *debug.BuildInfo) (string, string) {
	if strings.HasPrefix(bi.Main.Version, "v") {
		return fmt.Sprintf("bkl-%s", bi.Main.Version), "go-install"
	}

	for _, s := range bi.Settings {
		if s.Key != "-tags" {
			continue
		}

		tags := strings.Split(s.Value, ",")

		ver := ""
		src := "unknown"

		for _, tag := range tags {
			if strings.HasPrefix(tag, "bkl-v") {
				ver = tag
			} else if strings.HasPrefix(tag, "bkl-src-") {
				src = strings.TrimPrefix(tag, "bkl-src-")
			}
		}

		if ver != "" {
			return ver, src
		}
	}

	for _, s := range bi.Settings {
		if s.Key != "vcs.revision" {
			continue
		}

		return fmt.Sprintf("bkl-%s", s.Value[0:min(len(s.Value)-1, 10)]), "git"
	}

	return "bkl-[unknown]", "unknown"
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}
