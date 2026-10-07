package sysfs

import (
	"context"
	"fmt"
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
			if device.Type == DigitalInput {
				if !publishState(ctx, states, StateChange{Kind: InputFailureReportKind, Device: *device, Error: err}) {
					return
				}
			}
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

func publishInitialRelayStates(ctx context.Context, devices []*Device, states chan<- StateChange) bool {
	for _, device := range devices {
		if device.Type != RelayOutput {
			continue
		}
		value, err := readDevice(device)
		if err != nil {
			slog.Error("sysfs startup read failed", "device_id", device.Identifier, "path", device.Path, "error", err)
			continue
		}
		if !publishState(ctx, states, StateChange{
			Device:   *device,
			OldValue: value,
			NewValue: value,
			Initial:  true,
		}) {
			return false
		}
	}

	return true
}

func handleCommand(devicesByID map[string]*Device, cmd Command, ctx context.Context, states chan<- StateChange) {
	device, ok := devicesByID[cmd.DeviceID]
	if !ok {
		publishCompletion(ctx, states, cmd, time.Now(), fmt.Errorf("unknown device %q", cmd.DeviceID))
		return
	}

	oldValue, newValue, err := executeCommand(device, cmd)
	completedAt := time.Now()
	if err != nil {
		slog.Error("sysfs command failed", "device_id", cmd.DeviceID, "command_id", cmd.ID, "error", err)
	} else if oldValue != newValue {
		publishState(ctx, states, StateChange{
			Device: *device, OldValue: oldValue, NewValue: newValue, IsRising: oldValue == Off && newValue == On,
		})
	}
	publishCompletion(ctx, states, cmd, completedAt, err)
}

func executeCommand(device *Device, cmd Command) (Value, Value, error) {
	oldValue := device.Value
	var newValue Value
	switch cmd.Kind {
	case ToggleCommand:
		var err error
		oldValue, err = readCurrentValue(device)
		if err != nil {
			return oldValue, oldValue, err
		}
		newValue = On
		if oldValue == On {
			newValue = Off
		}
	case OnCommand:
		newValue = On
	case OffCommand:
		newValue = Off
	default:
		return oldValue, oldValue, fmt.Errorf("unsupported command %q", cmd.Kind)
	}
	if err := writeValue(device.Path, newValue); err != nil {
		return oldValue, oldValue, fmt.Errorf("write device %s: %w", device.Identifier, err)
	}
	device.Value = newValue
	return oldValue, newValue, nil
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

func publishCompletion(ctx context.Context, states chan<- StateChange, cmd Command, completedAt time.Time, err error) bool {
	return publishState(ctx, states, StateChange{
		Kind:       CompletionReportKind,
		Completion: &CommandCompletion{Command: cmd, CompletedAt: completedAt, Error: err},
	})
}

func publishState(ctx context.Context, states chan<- StateChange, state StateChange) bool {
	if ctx.Err() != nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case states <- state:
		return true
	}
}
