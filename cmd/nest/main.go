package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mhemeryck/nest/internal/nest"
)

func main() {
	configPath := flag.String("config", "", "Path to config file")
	validateOnly := flag.Bool("validate", false, "Validate config and exit")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := nest.Run(ctx, nest.Options{ConfigPath: *configPath, ValidateOnly: *validateOnly}); err != nil {
		slog.Error("run failed", "error", err)
		os.Exit(1)
	}
}
