package main

import (
	"os"

	"github.com/kayac/ecspresso"
)

var Version string

func main() {
	ecspresso.Version = Version
	os.Exit(ecspresso.Main(trapSignals))
}
