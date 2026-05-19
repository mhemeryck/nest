package sysfs

import (
	"context"
	"log/slog"
	"time"
)

func pollWorker(
	ctx context.Context,
	cfg WorkerConfig,
	commands <-chan Command,
	states chan<- StateChange,
) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	devicesByID := make(map[string]*Device, len(cfg.Devices))
	for _, device := range cfg.Devices {
		devicesByID[device.Identifier] = device
	}

	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-commands:
			if ctx.Err() != nil {
				return
			}
			handleCommand(devicesByID, cmd, ctx, states)
		case <-ticker.C:
			pollDevices(cfg.Devices, ctx, states)
		}
	}
}

func pollDevices(devices []*Device, ctx context.Context, states chan<- StateChange) {
	for _, device := range devices {
		oldValue := device.Value
		newValue, err := readDevice(device)
		if err != nil {
			continue
		}
		if newValue != oldValue {
			if !publishState(ctx, states, StateChange{
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

func handleCommand(devicesByID map[string]*Device, cmd Command, ctx context.Context, states chan<- StateChange) {
	device, ok := devicesByID[cmd.DeviceID]
	if !ok {
		return
	}

	switch cmd.Kind {
	case ToggleCommand:
		oldValue, err := readCurrentValue(device)
		if err != nil {
			return
		}
		newValue := On
		if oldValue == On {
			newValue = Off
		}
		writeCommandValue(device, oldValue, newValue, ctx, states)
	case OnCommand:
		oldValue, err := readCurrentValue(device)
		if err != nil {
			return
		}
		writeCommandValue(device, oldValue, On, ctx, states)
	case OffCommand:
		oldValue, err := readCurrentValue(device)
		if err != nil {
			return
		}
		writeCommandValue(device, oldValue, Off, ctx, states)
	default:
		return
	}
}

func readCurrentValue(device *Device) (Value, error) {
	oldValue, err := readValue(device.Path)
	if err != nil {
		slog.Error("sysfs read failed", "device_id", device.Identifier, "path", device.Path, "error", err)
		return Off, err
	}

	device.Value = oldValue
	return oldValue, nil
}

func writeCommandValue(device *Device, oldValue Value, newValue Value, ctx context.Context, states chan<- StateChange) {
	if err := writeValue(device.Path, newValue); err != nil {
		slog.Error("sysfs write failed", "device_id", device.Identifier, "path", device.Path, "error", err)
		return
	}

	device.Value = newValue
	if oldValue == newValue {
		return
	}

	publishState(ctx, states, StateChange{
		Device:   *device,
		OldValue: oldValue,
		NewValue: newValue,
		IsRising: oldValue == Off && newValue == On,
	})
}

func publishState(ctx context.Context, states chan<- StateChange, state StateChange) bool {
	select {
	case <-ctx.Done():
		return false
	case states <- state:
		return true
	}
}
