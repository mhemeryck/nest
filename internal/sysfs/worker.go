package sysfs

import "time"

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
			handleCommand(devicesByID, cmd, states)
		case <-ticker.C:
			pollDevices(cfg.Devices, states)
		}
	}
}

func pollDevices(devices []*Device, states chan<- PollEvent) {
	for _, device := range devices {
		oldValue := device.Value
		newValue, err := readDevice(device)
		if err != nil {
			continue
		}
		if newValue != oldValue {
			states <- PollEvent{
				Device:   *device,
				OldValue: oldValue,
				NewValue: newValue,
				IsRising: oldValue == Off && newValue == On,
			}
		}
	}
}

func handleCommand(devicesByID map[string]*Device, cmd Command, states chan<- PollEvent) {
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
			return
		}

		device.Value = newValue
		states <- PollEvent{
			Device:   *device,
			OldValue: oldValue,
			NewValue: newValue,
			IsRising: oldValue == Off && newValue == On,
		}
	default:
		return
	}
}
