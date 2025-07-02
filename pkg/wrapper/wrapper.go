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
			fatal(fmt.Errorf("ReadBuildInfo() failed")) //nolint:goerr113 // Dynamic error for build tool diagnostic output
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
		// Try to evaluate the file - Evaluate handles FileMatch internally
		fsys := os.DirFS("/")
		output, err := bkl.Evaluate(fsys, []string{arg}, "/", "", nil, nil, "", &arg)
		if err != nil {
			continue
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

		_, err = tmp.Write(output)
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
