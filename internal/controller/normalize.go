package controller

import (
	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func pushButtonEventsFromStateChange(index *registry.Registry, stateChange sysfs.StateChange) ([]event.Event, bool) {
	if stateChange.Kind != sysfs.ObservationReportKind {
		return nil, false
	}
	input, ok := registry.DigitalInputBySysfsDevice(index, entity.SysfsDeviceID(stateChange.Device.Identifier))
	if !ok {
		return nil, false
	}

	pushButtonKind := event.PushButtonReleasedKind
	if stateChange.IsRising {
		pushButtonKind = event.PushButtonPressedKind
	}

	buttons := registry.PushButtonsByInput(index, input.ID)
	buttonEvents := make([]event.Event, 0, len(buttons))
	for _, button := range buttons {
		buttonEvents = append(buttonEvents, event.Event{
			Kind: pushButtonKind,
			PushButton: &event.PushButton{
				ButtonID: button.ID,
				Name:     button.Name,
			},
		})
	}

	return buttonEvents, true
}

func semanticEventsFromStateChange(index *registry.Registry, stateChange sysfs.StateChange) ([]event.Event, bool) {
	deviceID := entity.SysfsDeviceID(stateChange.Device.Identifier)
	switch stateChange.Kind {
	case sysfs.CompletionReportKind:
		completion := stateChange.Completion
		if completion == nil {
			return nil, false
		}
		return []event.Event{{Kind: event.OutputResultKind, OutputResult: &event.OutputResult{
			CommandID: completion.Command.ID, SysfsDevice: entity.SysfsDeviceID(completion.Command.DeviceID),
			Action: entity.OutputAction(completion.Command.Kind), CompletedAt: completion.CompletedAt, Error: completion.Error,
		}}}, true
	case sysfs.InputFailureReportKind:
		input, ok := registry.DigitalInputBySysfsDevice(index, deviceID)
		if !ok {
			return nil, false
		}
		return []event.Event{{Kind: event.InputFailureKind, InputFailure: &event.InputFailure{
			InputID: input.ID, SysfsDevice: deviceID, Error: stateChange.Error,
		}}}, true
	case sysfs.ObservationReportKind:
	default:
		return nil, false
	}
	if input, ok := registry.DigitalInputBySysfsDevice(index, deviceID); ok {
		events := []event.Event{{
			Kind: event.DigitalInputStateKind,
			DigitalInput: &event.DigitalInput{
				InputID:     input.ID,
				SysfsDevice: input.SysfsDevice,
				Value:       sysfs.PrintableValue(stateChange.NewValue),
			},
		}}

		pushButtonEvents, _ := pushButtonEventsFromStateChange(index, stateChange)
		events = append(events, pushButtonEvents...)

		return events, true
	}

	if relay, ok := registry.RelayBySysfsDevice(index, deviceID); ok {
		events := []event.Event{{
			Kind: event.RelayStateKind,
			Relay: &event.Relay{
				RelayID:     relay.ID,
				Name:        relay.Name,
				SysfsDevice: relay.SysfsDevice,
				Value:       sysfs.PrintableValue(stateChange.NewValue),
			},
		}}

		for _, light := range registry.LightsByRelay(index, relay.ID) {
			events = append(events, event.Event{
				Kind: event.LightStateKind,
				LightState: &event.LightState{
					LightID: light.ID,
					Name:    light.Name,
					RelayID: relay.ID,
					Value:   sysfs.PrintableValue(stateChange.NewValue),
				},
			})
		}

		return events, true
	}

	return nil, false
}
