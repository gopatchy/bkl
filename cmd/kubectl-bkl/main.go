package main

import (
	"github.com/gopatchy/bkl/wrapper"
)

func main() {
	wrapper.WrapOrDie("kubectl")
}
