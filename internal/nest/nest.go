package nest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/controller"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
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
	root := entity.FromConfig(configRoot)
	index := registry.Build(root)

	// Resolve configured sysfs devices against the hardware tree before actors start.
	slog.Info("crawling sysfs device tree", "root", root.SysfsRoot)
	devices, err := sysfs.ListDevices(root.SysfsRoot)
	if err != nil {
		return fmt.Errorf("crawl sysfs: %w", err)
	}

	configuredDevices, missing := configuredDevices(devices, registry.SysfsDeviceIDs(index))
	if len(missing) > 0 {
		var errs error
		for _, deviceID := range missing {
			slog.Error("configured device not found in sysfs", "device_id", deviceID)
			errs = errors.Join(errs, fmt.Errorf("configured device %q not found in sysfs", deviceID))
		}
		return errs
	}

	slog.Info("configured devices", "count", len(configuredDevices))
	for _, device := range configuredDevices {
		slog.Info("configured device", "identifier", device.Identifier, "path", device.Path)
	}

	// Start optional transport actors before local control so startup state is published early.
	mqttCommands, mqttEvents, mqttDone := mqttChannels(root)
	if mqttCommands != nil {
		go mqtt.Run(ctx, root.MQTT, mqttCommands, mqttEvents, mqttDone)
	}

	// Start hardware and controller actors with unidirectional command and observation channels.
	sysfsCommands, states, sysfsDone := sysfsChannels()
	go sysfs.Run(ctx, configuredDevices, sysfsCommands, states, sysfsDone, sysfsPollIntervals(root))

	controllerDone := make(chan struct{})
	go controller.Run(ctx, root, index, sysfsCommands, mqttCommands, mqttTopics(root), states, mqttEvents, controllerDone)

	slog.Info("polling devices", "message", "press Ctrl+C to exit")

	// Controller shutdown drives process shutdown and then actors are drained in order.
	<-controllerDone
	cancel()
	<-sysfsDone
	close(states)
	if mqttDone != nil {
		<-mqttDone
	}
	if mqttEvents != nil {
		close(mqttEvents)
	}

	slog.Info("shutting down")
	return nil
}
