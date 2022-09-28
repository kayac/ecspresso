//go:build unix

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

var trapSignals = []os.Signal{os.Interrupt, unix.SIGTERM}
