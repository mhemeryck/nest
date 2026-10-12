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
	switch stateChange.Kind {
	case sysfs.CompletionReportKind:
		return semanticEventsFromCompletion(stateChange.Completion)
	case sysfs.InputFailureReportKind:
		return semanticEventsFromInputFailure(index, stateChange)
	case sysfs.ObservationReportKind:
		return semanticEventsFromObservation(index, stateChange)
	default:
		return nil, false
	}
}

func semanticEventsFromCompletion(completion *sysfs.CommandCompletion) ([]event.Event, bool) {
	if completion == nil {
		return nil, false
	}
	return []event.Event{{Kind: event.OutputResultKind, OutputResult: &event.OutputResult{
		CommandID: completion.Command.ID, SysfsDevice: entity.SysfsDeviceID(completion.Command.DeviceID),
		Action: entity.OutputAction(completion.Command.Kind), CompletedAt: completion.CompletedAt, Error: completion.Error,
	}}}, true
}

func semanticEventsFromInputFailure(index *registry.Registry, stateChange sysfs.StateChange) ([]event.Event, bool) {
	deviceID := entity.SysfsDeviceID(stateChange.Device.Identifier)
	input, ok := registry.DigitalInputBySysfsDevice(index, deviceID)
	if !ok {
		return nil, false
	}
	return []event.Event{{Kind: event.InputFailureKind, InputFailure: &event.InputFailure{
		InputID: input.ID, SysfsDevice: deviceID, Error: stateChange.Error,
	}}}, true
}

func semanticEventsFromObservation(index *registry.Registry, stateChange sysfs.StateChange) ([]event.Event, bool) {
	deviceID := entity.SysfsDeviceID(stateChange.Device.Identifier)
	if input, ok := registry.DigitalInputBySysfsDevice(index, deviceID); ok {
		return semanticEventsFromInputObservation(index, input, stateChange), true
	}

	if relay, ok := registry.RelayBySysfsDevice(index, deviceID); ok {
		return semanticEventsFromRelayObservation(index, relay, stateChange), true
	}

	return nil, false
}

func semanticEventsFromInputObservation(index *registry.Registry, input entity.DigitalInput, stateChange sysfs.StateChange) []event.Event {
	events := []event.Event{{
		Kind: event.DigitalInputStateKind,
		DigitalInput: &event.DigitalInput{
			InputID: input.ID, SysfsDevice: input.SysfsDevice,
			Value: sysfs.PrintableValue(stateChange.NewValue),
		},
	}}
	pushButtonEvents, _ := pushButtonEventsFromStateChange(index, stateChange)
	return append(events, pushButtonEvents...)
}

func semanticEventsFromRelayObservation(index *registry.Registry, relay entity.Relay, stateChange sysfs.StateChange) []event.Event {
	events := []event.Event{{
		Kind: event.RelayStateKind,
		Relay: &event.Relay{
			RelayID: relay.ID, Name: relay.Name, SysfsDevice: relay.SysfsDevice,
			Value: sysfs.PrintableValue(stateChange.NewValue),
		},
	}}
	for _, light := range registry.LightsByRelay(index, relay.ID) {
		events = append(events, event.Event{
			Kind: event.LightStateKind,
			LightState: &event.LightState{
				LightID: light.ID, Name: light.Name, RelayID: relay.ID,
				Value: sysfs.PrintableValue(stateChange.NewValue),
			},
		})
	}
	return events
}
