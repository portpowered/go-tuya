// Package main starts the standalone go-tuya command-line program.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/portpowered/go-tuya/cmd/go-tuya/internal/cli"
)

func main() {
	exitCode := runMain()
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func runMain() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var dependencies cli.Dependencies

	return cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, dependencies)
}
