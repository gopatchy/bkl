package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
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
	if os.Getenv("BKL_VERSION") != "" {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			fatal(fmt.Errorf("ReadBuildInfo() failed"))
		}

		fmt.Printf("%s", bi)
		os.Exit(0)
	}

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
		ext := filepath.Ext(arg)
		if !exts[ext] {
			continue
		}

		withoutExt := strings.TrimSuffix(arg, ext)
		foundPath := ""

		for tryExt := range exts {
			tryPath := withoutExt + tryExt

			_, err := os.Stat(tryPath)
			if err != nil {
				continue
			}

			foundPath = tryPath
			break
		}

		if foundPath == "" {
			continue
		}

		b := bkl.New()

		err = b.MergeFileLayers(foundPath)
		if err != nil {
			fatal(err)
		}

		pat := fmt.Sprintf(
			"%s.*.%s",
			filepath.Base(os.Args[0]),
			filepath.Base(arg),
		)

		tmp, err := os.CreateTemp("", pat)
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
