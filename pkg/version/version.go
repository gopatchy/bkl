package version

import (
	"fmt"
	"os"
	"runtime/debug"
)

func GetVersion() *debug.BuildInfo {
	bi, _ := debug.ReadBuildInfo()
	return bi
}

func PrintVersion(requested bool) {
	if !requested && os.Getenv("BKL_VERSION") == "" {
		return
	}

	bi := GetVersion()
	if bi == nil {
		fmt.Fprintf(os.Stderr, "ReadBuildInfo() failed\n")
		os.Exit(1)
	}

	fmt.Printf("%s", bi)
	os.Exit(0)
}
