package main

import (
	"github.com/gopatchy/bkl/pkg/wrapper"
)

func main() {
	wrapper.WrapOrDie("kubectl")
}
