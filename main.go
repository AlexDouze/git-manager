// main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexDouze/gitm/cmd"
)

func main() {
	// Cancel the context on interrupt/terminate so in-flight git and gh
	// subprocesses are killed promptly instead of hanging the CLI.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
