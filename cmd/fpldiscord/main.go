// Command fpldiscord is the single binary: a Discord bot goroutine plus an HTTP
// server sharing one in-memory snapshot of the Draft FPL API.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/UberJoe/fpldiscord/internal/app"
	"github.com/UberJoe/fpldiscord/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "boot failed:", err)
		os.Exit(1)
	}

	if err := a.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "run failed:", err)
		os.Exit(1)
	}
}
