package main

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"github.com/kayac/ecspresso"
)

var Version string

func main() {
	ecspresso.Version = Version
	ctx, stop := signal.NotifyContext(context.Background(), trapSignals...)
	defer stop()

	// switch cli parser
	parse := ecspresso.ParseCLI
	if v2 := os.Getenv("V2"); v2 != "" {
		parse = ecspresso.ParseCLIv2
	}

	exitCode, err := ecspresso.CLI(ctx, parse)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			ecspresso.Log("[WARNING] Interrupted")
		} else {
			ecspresso.Log("[ERROR] FAILED. %s", err)
		}
	}
	os.Exit(exitCode)
}
