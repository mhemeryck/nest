package sysfs

import (
	"log/slog"
	"time"
)

func pollWorker(
	cfg WorkerConfig,
	stopCh <-chan struct{},
	commands <-chan Command,
	states chan<- PollEvent,
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
			handleCommand(devicesByID, cmd, stopCh, states)
		case <-ticker.C:
			pollDevices(cfg.Devices, stopCh, states)
		}
	}
}

func pollDevices(devices []*Device, stopCh <-chan struct{}, states chan<- PollEvent) {
	for _, device := range devices {
		oldValue := device.Value
		newValue, err := readDevice(device)
		if err != nil {
			continue
		}
		if newValue != oldValue {
			if !publishState(stopCh, states, PollEvent{
				Device:   *device,
				OldValue: oldValue,
				NewValue: newValue,
				IsRising: oldValue == Off && newValue == On,
			}) {
				return
			}
		}
	}
}

func handleCommand(devicesByID map[string]*Device, cmd Command, stopCh <-chan struct{}, states chan<- PollEvent) {
	device, ok := devicesByID[cmd.DeviceID]
	if !ok {
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
			slog.Error("sysfs write failed", "device_id", device.Identifier, "path", device.Path, "error", err)
			return
		}

		device.Value = newValue
		publishState(stopCh, states, PollEvent{
			Device:   *device,
			OldValue: oldValue,
			NewValue: newValue,
			IsRising: oldValue == Off && newValue == On,
		})
	default:
		return
	}
}

func publishState(stopCh <-chan struct{}, states chan<- PollEvent, state PollEvent) bool {
	select {
	case <-stopCh:
		return false
	case states <- state:
		return true
	}
}
