package bkl

import (
	"log"
)

// debugLog logs a debug message if Debug is enabled
func debugLog(format string, v ...any) {
	if !Debug {
		return
	}

	log.Printf(format, v...)
}
