package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gopatchy/bkl/wrapper"
)

func main() {
	cmd := filepath.Base(os.Args[0])

	if strings.HasSuffix(cmd, "b") {
		cmd = strings.TrimSuffix(cmd, "b")
	} else {
		// Run as bklb, not via symlink
		//nolint:goerr113,revive,stylecheck
		fatal(fmt.Errorf(`Usage:
  ln -s $(which bklb) toolb  # bklb will run 'tool'

See https://bkl.gopatchy.io/#bklb for detailed documentation.`))
	}

	wrapper.WrapOrDie(cmd)
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
