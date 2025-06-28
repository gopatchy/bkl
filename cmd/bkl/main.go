package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"runtime/pprof"
	"strings"

	"github.com/gopatchy/bkl"
	"github.com/gopatchy/bkl/pkg/log"
	"github.com/jessevdk/go-flags"
	"golang.org/x/exp/constraints"
)

type options struct {
	OutputPath   *flags.Filename `short:"o" long:"output" description:"output file path"`
	OutputFormat *string         `short:"f" long:"format" description:"output format" choice:"json" choice:"json-pretty" choice:"jsonl" choice:"toml" choice:"yaml"`
	RootPath     string          `short:"r" long:"root-path" description:"restrict file access to this root directory" default:"/"`
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

	if opts.Version || os.Getenv("BKL_VERSION") != "" {
		version()
	}

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

	output, err := bkl.Evaluate(root.FS(), files, opts.RootPath, "", nil, opts.OutputFormat, (*string)(opts.OutputPath), &files[0])
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

func version() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		fatal(fmt.Errorf("ReadBuildInfo() failed"))
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
