package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/gopatchy/bkl"
)

var exts = map[string]bool{
	".json": true,
	".toml": true,
	".yaml": true,
}

func main() {
	cmd := strings.TrimSuffix(filepath.Base(os.Args[0]), "b")
	args := slices.Clone(os.Args[1:])

	if cmd == "bkl" {
		// Run as bklb, not via symlink
		fatal(fmt.Errorf("usage: ln -s `which bklb` toolb  # bklb will run 'tool'"))
	}

	cmdPath, err := exec.LookPath(cmd)
	if err != nil {
		fatal(err)
	}

	for i, arg := range args {
		_, err := os.Stat(arg)
		if err != nil {
			continue
		}

		if !exts[filepath.Ext(arg)] {
			continue
		}

		b := bkl.New()

		err = b.MergeFileLayers(arg)
		if err != nil {
			fatal(err)
		}

		tmp, err := os.CreateTemp("", filepath.Base(os.Args[0]))
		if err != nil {
			fatal(err)
		}

		err = b.OutputToFile(tmp.Name(), strings.TrimPrefix(filepath.Ext(arg), "."))
		if err != nil {
			fatal(err)
		}

		args[i] = tmp.Name()

		tmp.Close()
	}

	fatal(syscall.Exec(cmdPath, append([]string{cmd}, args...), os.Environ()))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}
