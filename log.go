package bkl

import (
	"log"
)

func debugLog(format string, v ...any) {
	if !Debug {
		return
	}

	log.Printf(format, v...)
}
