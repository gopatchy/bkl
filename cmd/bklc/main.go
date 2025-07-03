package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gopatchy/bkl"
	"github.com/jessevdk/go-flags"
)

type options struct {
	Format   *string `short:"f" long:"format" description:"output format" choice:"json" choice:"jsonl" choice:"toml" choice:"yaml"`
	SortPath string  `short:"s" long:"sort" description:"sort output documents by path (e.g. 'metadata.name')"`
	Color    bool    `short:"c" long:"color" description:"colorize diff output"`

	Positional struct {
		File1 flags.Filename `positional-arg-name:"file1" required:"yes" description:"first file to compare"`
		File2 flags.Filename `positional-arg-name:"file2" required:"yes" description:"second file to compare"`
	} `positional-args:"yes"`
}

func main() {
	opts := &options{}

	fp := flags.NewParser(opts, flags.Default)
	fp.LongDescription = `bklc compares two bkl files and shows text differences between their outputs.

Examples:
  bklc base.yaml prod.yaml
  bklc -f yaml base.yaml prod.yaml
  bklc -c base.yaml prod.yaml`

	_, err := fp.Parse()
	if err != nil {
		if flags.WroteHelp(err) {
			os.Exit(0)
		}
		os.Exit(1)
	}

	file1 := string(opts.Positional.File1)
	file2 := string(opts.Positional.File2)

	fsys := os.DirFS("/")

	result, err := bkl.Compare(fsys, file1, file2, "/", "", nil, opts.Format, opts.SortPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if opts.Color {
		fmt.Print(colorizeDiff(result.Diff))
	} else {
		fmt.Print(result.Diff)
	}
}

func colorizeDiff(diff string) string {
	const (
		red   = "\033[31m"
		green = "\033[32m"
		cyan  = "\033[36m"
		reset = "\033[0m"
	)

	lines := strings.Split(diff, "\n")
	var result []string

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			result = append(result, cyan+line+reset)
		case strings.HasPrefix(line, "@@"):
			result = append(result, cyan+line+reset)
		case strings.HasPrefix(line, "-"):
			result = append(result, red+line+reset)
		case strings.HasPrefix(line, "+"):
			result = append(result, green+line+reset)
		default:
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}
