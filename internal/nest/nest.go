package nest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/controller"
	"github.com/mhemeryck/nest/internal/entity"
	nestmqtt "github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

type Options struct {
	ConfigPath   string
	ValidateOnly bool
}

func Run(ctx context.Context, opts Options) error {
	configRoot, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if opts.ValidateOnly {
		slog.Info("config is valid", "path", opts.ConfigPath)
		return nil
	}

	root := entity.FromConfig(configRoot)
	index := registry.Build(root)

	slog.Info("crawling sysfs device tree", "root", root.SysfsRoot)
	devices, err := sysfs.ListDevices(root.SysfsRoot)
	if err != nil {
		return fmt.Errorf("crawl sysfs: %w", err)
	}

	configuredDevices, missing := configuredDevices(devices, registry.DeviceIDs(index))
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	commands := make(chan sysfs.Command, 32)
	states := make(chan sysfs.StateChange, 32)
	var mqttStates chan nestmqtt.State
	var mqttDone chan struct{}
	if root.MQTT.Enabled {
		mqttConfig := mqttConfigFromEntity(root.MQTT)
		discovery := nestmqtt.DiscoveryStates(mqttConfig, root, index)
		mqttStates = make(chan nestmqtt.State, 64)
		mqttCommands := make(chan nestmqtt.Command, 1)
		mqttDone = make(chan struct{})
		go nestmqtt.Run(ctx, mqttConfig, discovery, mqttStates, mqttCommands, mqttDone)
	}

	sysfsDone := make(chan struct{})
	go sysfs.Run(ctx, configuredDevices, commands, states, sysfsDone)

	controllerDone := make(chan struct{})
	go controller.Run(ctx, index, commands, mqttStates, states, controllerDone)

	slog.Info("polling devices", "message", "press Ctrl+C to exit")

	<-controllerDone
	cancel()
	<-sysfsDone
	close(states)
	if mqttStates != nil {
		close(mqttStates)
		<-mqttDone
	}

	slog.Info("shutting down")
	return nil
}
