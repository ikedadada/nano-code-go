package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	a2ahttp "nano-code-go/internal/interfaces/a2a"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := a2ahttp.Run(ctx, os.Stdout, os.Stderr, a2ahttp.EnvFromOS()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
