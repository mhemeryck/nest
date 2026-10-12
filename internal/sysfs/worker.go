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

	devicesByID := devicesByIdentifier(cfg.Devices)

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

func devicesByIdentifier(devices []*Device) map[string]*Device {
	index := make(map[string]*Device, len(devices))
	for _, device := range devices {
		index[device.Identifier] = device
	}
	return index
}

func pollDevices(devices []*Device, ctx context.Context, states chan<- StateChange) {
	for _, device := range devices {
		if !pollDevice(device, ctx, states) {
			return
		}
	}
}

func pollDevice(device *Device, ctx context.Context, states chan<- StateChange) bool {
	oldValue := device.Value
	newValue, err := readDevice(device)
	if err != nil {
		if device.Type == DigitalInput {
			return publishState(ctx, states, StateChange{Kind: InputFailureReportKind, Device: *device, Error: err})
		}
		return true
	}
	if newValue == oldValue {
		return true
	}
	return publishState(ctx, states, StateChange{
		Device: *device, OldValue: oldValue, NewValue: newValue,
		IsRising: oldValue == Off && newValue == On,
	})
}

func publishInitialRelayStates(ctx context.Context, devices []*Device, states chan<- StateChange) bool {
	for _, device := range devices {
		if device.Type != RelayOutput {
			continue
		}
		if !publishInitialRelayState(ctx, device, states) {
			return false
		}
	}

	return true
}

func publishInitialRelayState(ctx context.Context, device *Device, states chan<- StateChange) bool {
	value, err := readDevice(device)
	if err != nil {
		slog.Error("sysfs startup read failed", "device_id", device.Identifier, "path", device.Path, "error", err)
		return true
	}
	return publishState(ctx, states, StateChange{
		Device: *device, OldValue: value, NewValue: value, Initial: true,
	})
}

type commandExecution struct {
	oldValue    Value
	newValue    Value
	completedAt time.Time
	err         error
}

func handleCommand(devicesByID map[string]*Device, cmd Command, ctx context.Context, states chan<- StateChange) {
	device, ok := devicesByID[cmd.DeviceID]
	if !ok {
		publishCompletion(ctx, states, cmd, time.Now(), fmt.Errorf("unknown device %q", cmd.DeviceID))
		return
	}

	execution := executeReportedCommand(device, cmd)
	publishCommandObservation(ctx, states, device, cmd, execution)
	publishCompletion(ctx, states, cmd, execution.completedAt, execution.err)
}

func executeReportedCommand(device *Device, cmd Command) commandExecution {
	oldValue, newValue, err := executeCommand(device, cmd)
	return commandExecution{oldValue: oldValue, newValue: newValue, completedAt: time.Now(), err: err}
}

func publishCommandObservation(ctx context.Context, states chan<- StateChange, device *Device, cmd Command, execution commandExecution) {
	if execution.err != nil {
		slog.Error("sysfs command failed", "device_id", cmd.DeviceID, "command_id", cmd.ID, "error", execution.err)
	} else if execution.oldValue != execution.newValue {
		publishState(ctx, states, StateChange{
			Device: *device, OldValue: execution.oldValue, NewValue: execution.newValue,
			IsRising: execution.oldValue == Off && execution.newValue == On,
		})
	}
}

func executeCommand(device *Device, cmd Command) (Value, Value, error) {
	oldValue, newValue, err := resolveCommandValues(device, cmd.Kind)
	if err != nil {
		return oldValue, oldValue, err
	}
	if err := writeDeviceValue(device, newValue); err != nil {
		return oldValue, oldValue, err
	}
	return oldValue, newValue, nil
}

func writeDeviceValue(device *Device, value Value) error {
	if err := writeValue(device.Path, value); err != nil {
		return fmt.Errorf("write device %s: %w", device.Identifier, err)
	}
	device.Value = value
	return nil
}

func resolveCommandValues(device *Device, kind CommandKind) (Value, Value, error) {
	oldValue := device.Value
	var newValue Value
	switch kind {
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
		return oldValue, oldValue, fmt.Errorf("unsupported command %q", kind)
	}
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
