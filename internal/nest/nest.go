package nest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/controller"
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
	persistenceActor := newPersistenceActor(reg)
	actorContext, cancelActors := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelActors()

	startMQTTActor(actorContext, mqttActor)
	startSysfsActor(actorContext, sysfsActor)
	startModbusActor(actorContext, modbusActor)
	startPersistenceActor(actorContext, persistenceActor)

	controllerDone := startController(ctx, reg, sysfsActor, mqttActor, modbusActor, controller.RuntimeOptions{
		Persistence: persistenceActor.store, RestoredPositions: persistenceActor.positions, FeedbackContext: actorContext,
	})

	slog.Info("runtime started", "message", "press Ctrl+C to exit")

	err = waitForShutdown(ctx, cancelActors, controllerDone, sysfsActor, mqttActor, modbusActor, shutdownOptions{
		cancelController: cancel, period: registry.CoverControl(reg).ShutdownPeriod, persistence: persistenceActor,
	})

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
