package nest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/entity"
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

	root, index, err := loadConfig(opts)
	if err != nil {
		return err
	}

	if opts.ValidateOnly {
		slog.Info("config is valid", "path", opts.ConfigPath, "unit_id", opts.UnitID)
		return nil
	}

	mqttActor := newMQTTActor(root)
	sysfsActor, err := newSysfsActor(root, index)
	if err != nil {
		return err
	}
	logModbusConfig(root)

	startMQTTActor(ctx, mqttActor)
	startSysfsActor(ctx, sysfsActor)

	controllerDone := startController(ctx, root, index, sysfsActor, mqttActor)

	slog.Info("runtime started", "message", "press Ctrl+C to exit")

	waitForShutdown(cancel, controllerDone, sysfsActor, mqttActor)

	slog.Info("shutting down")
	return nil
}

func loadConfig(opts Options) (*entity.Root, *registry.Registry, error) {
	configRoot, err := config.Load(opts.ConfigPath, opts.UnitID)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}

	root := config.ToEntityRoot(configRoot)
	index := registry.Build(root)

	return root, index, nil
}
