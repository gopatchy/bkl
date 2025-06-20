package wrapper

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"syscall"

	"github.com/gopatchy/bkl"
	"golang.org/x/exp/slices"
)

func WrapOrDie(cmd string) {
	if os.Getenv("BKL_VERSION") != "" {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			fatal(fmt.Errorf("ReadBuildInfo() failed")) //nolint:goerr113
		}

		fmt.Printf("%s", bi)
		os.Exit(0)
	}

	cmdPath, err := exec.LookPath(cmd)
	if err != nil {
		fatal(err)
	}

	args := slices.Clone(os.Args[1:])

	for i, arg := range args {
		b, err := bkl.New()
		if err != nil {
			fatal(err)
		}

		realPath, f, err := b.FileMatch(arg)
		if err != nil {
			continue
		}

		err = b.MergeFileLayers(realPath)
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

		err = b.OutputToFile(tmp.Name(), f)
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
