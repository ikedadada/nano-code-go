package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"nano-code-go/internal/interfaces/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, cli.EnvFromOS()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
