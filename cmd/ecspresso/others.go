//go:build !unix

package main

import (
	"os"
)

var trapSignals = []os.Signal{os.Interrupt}
