package log

import (
	"log"
	"os"
)

// Debug controls debug log output. Set by BKL_DEBUG environment variable by default.
var Debug = os.Getenv("BKL_DEBUG") != ""

// Debugf logs a debug message if Debug is true.
func Debugf(format string, v ...any) {
	if !Debug {
		return
	}

	log.Printf(format, v...)
}
