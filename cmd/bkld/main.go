package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/gopatchy/bkl"
	"github.com/jessevdk/go-flags"
)

type options struct {
	OutputPath   *flags.Filename `short:"o" long:"output" description:"output file path"`
	OutputFormat *string         `short:"f" long:"format" description:"output format" choice:"json" choice:"json-pretty" choice:"toml" choice:"yaml"`

	Positional struct {
		BasePath   flags.Filename `positional-arg-name:"basePath" required:"true" description:"base layer file path"`
		TargetPath flags.Filename `positional-arg-name:"targetPath" required:"true" description:"target output file path"`
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
bkld generates the minimal intermediate layer needed to create the target output from the base layer.

See https://bkl.gopatchy.io/#bkld for detailed documentation.`

	_, err := fp.Parse()
	if err != nil {
		os.Exit(1)
	}

	format := ""

	if opts.OutputFormat != nil {
		format = *opts.OutputFormat
	}

	if format == "" && opts.OutputPath != nil {
		format = strings.TrimPrefix(filepath.Ext(string(*opts.OutputPath)), ".")
	}

	baseDoc, f, err := getOnlyDocument(string(opts.Positional.BasePath))
	if err != nil {
		fatal(err)
	}

	baseDocs, err := baseDoc.Process([]*bkl.Document{baseDoc})
	if err != nil {
		fatal(err)
	}

	if len(baseDocs) != 1 {
		fatal(fmt.Errorf("bkld operates on exactly 1 source document per file"))
	}

	baseDoc = baseDocs[0]

	if format == "" {
		format = f
	}

	targetDoc, _, err := getOnlyDocument(string(opts.Positional.TargetPath))
	if err != nil {
		fatal(err)
	}

	targetDocs, err := targetDoc.Process([]*bkl.Document{targetDoc})
	if err != nil {
		fatal(err)
	}

	if len(targetDocs) != 1 {
		fatal(fmt.Errorf("bkld operates on exactly 1 source document per file"))
	}

	targetDoc = targetDocs[0]

	doc, err := diffDoc(targetDoc, baseDoc)
	if err != nil {
		fatal(err)
	}

	outF, err := bkl.GetFormat(format)
	if err != nil {
		fatal(err)
	}

	enc, err := outF.MarshalStream([]any{doc})
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

func getOnlyDocument(path string) (*bkl.Document, string, error) {
	b, err := bkl.New()
	if err != nil {
		return nil, "", err
	}

	rebasedPaths, err := bkl.PreparePathsForParserFromCwd([]string{path}, "/")
	if err != nil {
		return nil, "", err
	}

	realPath, f, err := b.FileMatch(rebasedPaths[0])
	if err != nil {
		return nil, "", err
	}

	err = b.MergeFileLayers(realPath)
	if err != nil {
		return nil, "", err
	}

	docs := b.Documents()

	if len(docs) != 1 {
		return nil, "", fmt.Errorf("bkld operates on exactly 1 source document per file")
	}

	return docs[0], f, nil
}
