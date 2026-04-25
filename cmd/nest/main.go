package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/mhemeryck/nest/internal/config"
	"github.com/mhemeryck/nest/internal/controller"
	"github.com/mhemeryck/nest/internal/entity"
	nestmqtt "github.com/mhemeryck/nest/internal/mqtt"
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
		os.Exit(1)
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

	commands := make(chan sysfs.Command, 32)
	states := make(chan sysfs.StateChange, 32)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	mqttStates, mqttDone := startMQTT(ctx, root, index)
	sysfsDone := make(chan struct{})
	go sysfs.Run(ctx, configuredDevices, commands, states, sysfsDone)

	controllerDone := make(chan struct{})
	go controller.Run(ctx, index, commands, mqttStates, states, controllerDone)

	slog.Info("polling devices", "message", "press Ctrl+C to exit")

	<-controllerDone
	stop()
	<-sysfsDone
	close(states)
	if mqttStates != nil {
		close(mqttStates)
		<-mqttDone
	}

	slog.Info("shutting down")
}

func startMQTT(ctx context.Context, root *entity.Root, index *registry.Index) (chan nestmqtt.State, chan struct{}) {
	if !root.MQTT.Enabled {
		return nil, nil
	}

	cfg := mqttConfig(root.MQTT)
	discoveryStates := nestmqtt.DiscoveryStates(cfg, root, index)
	states := make(chan nestmqtt.State, len(discoveryStates)+64)
	commands := make(chan nestmqtt.Command, 1)
	done := make(chan struct{})
	go nestmqtt.Run(ctx, cfg, states, commands, done)

	for _, state := range discoveryStates {
		states <- state
	}

	return states, done
}

func mqttConfig(cfg entity.MQTT) nestmqtt.Config {
	return nestmqtt.Config{
		Enabled:         cfg.Enabled,
		Broker:          cfg.Broker,
		UnitID:          cfg.UnitID,
		ClientID:        cfg.ClientID,
		Username:        cfg.Username,
		Password:        cfg.Password,
		DiscoveryPrefix: cfg.DiscoveryPrefix,
	}
}

func configuredDevices(devices []*sysfs.Device, wanted []entity.DeviceID) ([]*sysfs.Device, []entity.DeviceID) {
	byIdentifier := make(map[string]*sysfs.Device, len(devices))
	for _, device := range devices {
		byIdentifier[device.Identifier] = device
	}

	configured := make([]*sysfs.Device, 0, len(wanted))
	missing := make([]entity.DeviceID, 0)
	for _, identifier := range wanted {
		device, ok := byIdentifier[string(identifier)]
		if !ok {
			missing = append(missing, identifier)
			continue
		}
		configured = append(configured, device)
	}

	slices.Sort(missing)

	return configured, missing
}
