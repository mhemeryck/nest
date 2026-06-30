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

	// Load and validate external configuration before building runtime state.
	configRoot, err := config.Load(opts.ConfigPath, opts.UnitID)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if opts.ValidateOnly {
		slog.Info("config is valid", "path", opts.ConfigPath, "unit_id", opts.UnitID)
		return nil
	}

	// Translate config into domain entities and indexes used by controllers.
	root := config.ToEntityRoot(configRoot)
	index := registry.Build(root)

	configuredDevices, err := sysfsDevices(root, index)
	if err != nil {
		return err
	}
	logSysfsDevices(configuredDevices)
	logModbusConfig(root)

	mqttActor := newMQTTActor(root)
	sysfsActor := newSysfsActor(root, configuredDevices)

	startMQTTActor(ctx, mqttActor)
	startSysfsActor(ctx, sysfsActor)

	controllerDone := startController(ctx, root, index, sysfsActor, mqttActor)

	slog.Info("runtime started", "message", "press Ctrl+C to exit")

	waitForShutdown(cancel, controllerDone, sysfsActor, mqttActor)

	slog.Info("shutting down")
	return nil
}
