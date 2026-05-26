package main

import (
	"log/slog"
	"os"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		slog.Error("run failed", "error", err)
		os.Exit(1)
	}
}
