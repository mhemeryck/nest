package nest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/registry"
)

type Options struct {
	ConfigPath   string
	UnitID       string
	ValidateOnly bool
}

func Run(ctx context.Context, opts Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	reg, err := loadRegistry(opts)
	if err != nil {
		return err
	}

	if opts.ValidateOnly {
		slog.Info("config is valid", "path", opts.ConfigPath, "unit_id", opts.UnitID)
		return nil
	}

	actors, err := newRuntimeActors(reg)
	if err != nil {
		return err
	}
	actorContext, cancelActors := startRuntimeActors(ctx, actors)
	defer cancelActors()
	controllerDone := startRuntimeController(ctx, actorContext, reg, actors)

	slog.Info("runtime started", "message", "press Ctrl+C to exit")

	err = waitForRuntimeShutdown(ctx, cancel, cancelActors, controllerDone, reg, actors)

	slog.Info("shutting down")
	return err
}

func loadRegistry(opts Options) (*registry.Registry, error) {
	configRoot, err := config.Load(opts.ConfigPath, opts.UnitID)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	root := config.ToEntityRoot(configRoot)
	reg := registry.Build(root)

	return reg, nil
}
