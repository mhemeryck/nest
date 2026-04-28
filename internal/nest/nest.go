package nest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/controller"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

type Options struct {
	ConfigPath   string
	ValidateOnly bool
}

func Run(ctx context.Context, opts Options) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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

	commands := make(chan sysfs.Command, 32)
	states := make(chan sysfs.StateChange, 32)
	sysfsDone := make(chan struct{})
	go sysfs.Run(ctx, configuredDevices, commands, states, sysfsDone)

	controllerDone := make(chan struct{})
	go controller.Run(ctx, index, commands, states, controllerDone)

	slog.Info("polling devices", "message", "press Ctrl+C to exit")

	<-controllerDone
	cancel()
	<-sysfsDone
	close(states)

	slog.Info("shutting down")
	return nil
}
