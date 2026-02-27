package main

import (
	"context"
	"os"

	"neabrain/internal/adapters/inbound/cli"
)

func main() {
	ctx := context.Background()
	exitCode := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
