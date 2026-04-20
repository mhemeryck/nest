package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/event"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func main() {
	configPath := flag.String("config", "", "Path to config file")
	validateOnly := flag.Bool("validate", false, "Validate config and exit")
	flag.Parse()

	configRoot, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	if *validateOnly {
		slog.Info("config is valid", "path", *configPath)
		return
	}

	root := entity.FromConfig(configRoot)
	index := registry.Build(root)

	slog.Info("crawling sysfs device tree", "root", root.SysfsRoot)
	devices, err := sysfs.ListDevices(root.SysfsRoot)
	if err != nil {
		slog.Error("crawl failed", "error", err)
		return
	}

	configuredDevices, missing := configuredDevices(devices, registry.DeviceIDs(index))
	if len(missing) > 0 {
		for _, deviceID := range missing {
			slog.Error("configured device not found in sysfs", "device_id", deviceID)
		}
		os.Exit(1)
	}

	slog.Info("configured devices", "count", len(configuredDevices))
	for _, device := range configuredDevices {
		slog.Info("configured device", "identifier", device.Identifier, "path", device.Path)
	}

	configs := sysfs.BuildWorkerConfigs(configuredDevices)

	stopChs, pollEvents := sysfs.StartWorkers(configs)

	slog.Info("polling devices", "message", "press Ctrl+C to exit")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	dispatch(index, pollEvents, sigCh)

	slog.Info("shutting down")
	sysfs.StopWorkers(stopChs)
}

func dispatch(
	index *registry.Index,
	pollEvents <-chan sysfs.PollEvent,
	sigCh <-chan os.Signal,
) {
	for {
		select {
		case <-sigCh:
			return
		case pollEvent, ok := <-pollEvents:
			if !ok {
				return
			}

			handlePollEvent(index, pollEvent)
		}
	}
}

func handlePollEvent(index *registry.Index, pollEvent sysfs.PollEvent) {
	digitalInputEvent, ok := event.PollEventToDigitalInputEvent(index, pollEvent)
	if ok {
		handleEvent(index, digitalInputEvent)
		return
	}

	slog.Info(
		"poll event",
		"identifier",
		pollEvent.Device.Identifier,
		"path",
		pollEvent.Device.Path,
		"old_value",
		int(pollEvent.OldValue-'0'),
		"new_value",
		int(pollEvent.NewValue-'0'),
		"rising",
		pollEvent.IsRising,
	)
}

func handleEvent(index *registry.Index, busEvent event.Event) {
	switch busEvent.Kind {
	case event.DigitalInputKind:
		slog.Info(
			"digital input event",
			"input_id",
			busEvent.DigitalInput.InputID,
			"device_id",
			busEvent.DigitalInput.DeviceID,
			"rising",
			busEvent.DigitalInput.IsRising,
			"falling",
			busEvent.DigitalInput.IsFalling,
		)

		for _, pushButtonEvent := range event.DigitalInputEventToPushButtonEvents(index, *busEvent.DigitalInput) {
			handleEvent(index, pushButtonEvent)
		}
	case event.PushButtonKind:
		slog.Info(
			"push button event",
			"button_id",
			busEvent.PushButton.ButtonID,
			"name",
			busEvent.PushButton.Name,
			"kind",
			busEvent.PushButton.Kind,
		)
	}
}

func configuredDevices(devices []*sysfs.Device, wanted []string) ([]*sysfs.Device, []string) {
	byIdentifier := make(map[string]*sysfs.Device, len(devices))
	for _, device := range devices {
		byIdentifier[device.Identifier] = device
	}

	configured := make([]*sysfs.Device, 0, len(wanted))
	missing := make([]string, 0)
	for _, identifier := range wanted {
		device, ok := byIdentifier[identifier]
		if !ok {
			missing = append(missing, identifier)
			continue
		}
		configured = append(configured, device)
	}

	sort.Strings(missing)
	return configured, missing
}
