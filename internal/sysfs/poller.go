package sysfs

import (
	"fmt"
	"sync"
	"time"
)

type WorkerConfig struct {
	Interval time.Duration
	Devices  []*Device
}

type PollEvent struct {
	Device   Device
	OldValue Value
	NewValue Value
	IsRising bool
}

type CommandKind string

const ToggleCommand CommandKind = "toggle"

type Command struct {
	Kind     CommandKind
	DeviceID string
	Result   chan CommandResult
}

type CommandResult struct {
	DeviceID string
	Value    Value
	Err      error
}

func StartWorkers(configs []WorkerConfig) (chan<- Command, <-chan PollEvent, chan<- struct{}, <-chan struct{}) {
	commands := make(chan Command, 32)
	events := make(chan PollEvent, 32)
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})

	deviceRoutes := make(map[string]chan Command)
	var wg sync.WaitGroup

	for _, cfg := range configs {
		commandCh := make(chan Command, 32)
		for _, device := range cfg.Devices {
			deviceRoutes[device.Identifier] = commandCh
		}

		wg.Add(1)
		go func(cfg WorkerConfig, commandCh <-chan Command) {
			defer wg.Done()
			pollWorker(cfg, stopCh, commandCh, events)
		}(cfg, commandCh)
	}

	go routeCommands(stopCh, commands, deviceRoutes)

	go func() {
		wg.Wait()
		close(events)
		close(doneCh)
	}()

	return commands, events, stopCh, doneCh
}

func StopWorkers(stopCh chan<- struct{}, doneCh <-chan struct{}) {
	close(stopCh)
	<-doneCh
}

func routeCommands(stopCh <-chan struct{}, commands <-chan Command, deviceRoutes map[string]chan Command) {
	for {
		select {
		case <-stopCh:
			return
		case cmd, ok := <-commands:
			if !ok {
				return
			}

			commandCh, found := deviceRoutes[cmd.DeviceID]
			if !found {
				respondCommand(cmd, CommandResult{DeviceID: cmd.DeviceID, Err: fmt.Errorf("unknown device %q", cmd.DeviceID)})
				continue
			}

			select {
			case <-stopCh:
				respondCommand(cmd, CommandResult{DeviceID: cmd.DeviceID, Err: fmt.Errorf("sysfs workers stopped")})
			case commandCh <- cmd:
			}
		}
	}
}

func pollWorker(
	cfg WorkerConfig,
	stopCh <-chan struct{},
	commands <-chan Command,
	events chan<- PollEvent,
) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	devicesByID := make(map[string]*Device, len(cfg.Devices))
	for _, device := range cfg.Devices {
		devicesByID[device.Identifier] = device
	}

	for {
		select {
		case <-stopCh:
			return
		case cmd := <-commands:
			handleCommand(devicesByID, cmd, events)
		case <-ticker.C:
			for _, device := range cfg.Devices {
				oldValue := device.Value
				newValue, err := readDevice(device)
				if err != nil {
					continue
				}
				if newValue != oldValue {
					events <- PollEvent{
						Device:   *device,
						OldValue: oldValue,
						NewValue: newValue,
						IsRising: oldValue == Off && newValue == On,
					}
				}
			}
		}
	}
}

func handleCommand(devicesByID map[string]*Device, cmd Command, events chan<- PollEvent) {
	device, ok := devicesByID[cmd.DeviceID]
	if !ok {
		respondCommand(cmd, CommandResult{DeviceID: cmd.DeviceID, Err: fmt.Errorf("unknown device %q", cmd.DeviceID)})
		return
	}

	switch cmd.Kind {
	case ToggleCommand:
		oldValue := device.Value
		newValue := On
		if oldValue == On {
			newValue = Off
		}

		if err := writeValue(device.Path, newValue); err != nil {
			respondCommand(cmd, CommandResult{DeviceID: cmd.DeviceID, Err: fmt.Errorf("write device %s: %w", device.Identifier, err)})
			return
		}

		device.Value = newValue
		events <- PollEvent{
			Device:   *device,
			OldValue: oldValue,
			NewValue: newValue,
			IsRising: oldValue == Off && newValue == On,
		}
		respondCommand(cmd, CommandResult{DeviceID: cmd.DeviceID, Value: newValue})
	default:
		respondCommand(cmd, CommandResult{DeviceID: cmd.DeviceID, Err: fmt.Errorf("unsupported command %q", cmd.Kind)})
	}
}

func respondCommand(cmd Command, result CommandResult) {
	if cmd.Result == nil {
		return
	}

	cmd.Result <- result
}

func BuildWorkerConfigs(devices []*Device) []WorkerConfig {
	configs := make(map[DeviceType]WorkerConfig)

	defaultIntervals := map[DeviceType]time.Duration{
		DigitalInput:  20 * time.Millisecond,
		DigitalOutput: 100 * time.Millisecond,
		RelayOutput:   1 * time.Second,
	}

	for _, d := range devices {
		cfg, ok := configs[d.Type]
		if !ok {
			cfg = WorkerConfig{Interval: defaultIntervals[d.Type]}
		}
		cfg.Devices = append(cfg.Devices, d)
		configs[d.Type] = cfg
	}

	result := make([]WorkerConfig, 0, len(configs))
	for _, cfg := range configs {
		result = append(result, cfg)
	}
	return result
}
