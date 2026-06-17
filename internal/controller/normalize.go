package controller

import (
	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func pushButtonEventsFromStateChange(index *registry.Index, stateChange sysfs.StateChange) ([]event.Event, bool) {
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

func semanticEventsFromStateChange(index *registry.Index, stateChange sysfs.StateChange) ([]event.Event, bool) {
	deviceID := entity.SysfsDeviceID(stateChange.Device.Identifier)
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
