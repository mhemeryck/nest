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

	mqttActor := newMQTTActor(reg)
	sysfsActor, err := newSysfsActor(reg)
	if err != nil {
		return err
	}
	modbusActor := newModbusActor(reg)

	startMQTTActor(ctx, mqttActor)
	startSysfsActor(ctx, sysfsActor)
	startModbusActor(ctx, modbusActor)

	controllerDone := startController(ctx, reg, sysfsActor, mqttActor, modbusActor)

	slog.Info("runtime started", "message", "press Ctrl+C to exit")

	waitForShutdown(cancel, controllerDone, sysfsActor, mqttActor, modbusActor)

	slog.Info("shutting down")
	return nil
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
