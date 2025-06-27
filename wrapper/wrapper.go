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
		// Prepare the path for FileMatch
		preparedPaths, err := bkl.PreparePathsFromCwd([]string{arg}, "/")
		if err != nil {
			continue
		}

		fsys := os.DirFS("/")
		realPath, f, err := bkl.FileMatch(fsys, preparedPaths[0])
		if err != nil {
			continue
		}

		// Get current working directory for Evaluate
		wd, err := os.Getwd()
		if err != nil {
			fatal(err)
		}

		// Use Evaluate to process the file
		output, err := bkl.Evaluate(fsys, []string{realPath}, "/", wd, nil, &f)
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

		// Write the output to the temp file
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
