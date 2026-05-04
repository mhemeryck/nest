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

	pushButtonKind := event.PushButtonReleased
	if stateChange.IsRising {
		pushButtonKind = event.PushButtonPressed
	}

	buttons := registry.PushButtonsByInput(index, input.ID)
	buttonEvents := make([]event.Event, 0, len(buttons))
	for _, button := range buttons {
		buttonEvents = append(buttonEvents, event.Event{
			Kind: event.PushButtonKind,
			PushButton: &event.PushButton{
				ButtonID: button.ID,
				Name:     button.Name,
				Kind:     pushButtonKind,
			},
		})
	}

	return buttonEvents, true
}

func lightEventsFromPushButton(index *registry.Index, pushButton event.PushButton) []event.Event {
	if pushButton.Kind != event.PushButtonPressed {
		return nil
	}

	bindings := registry.BindingsByButton(index, pushButton.ButtonID)
	lightEvents := make([]event.Event, 0, len(bindings))
	for _, binding := range bindings {
		name := ""
		if light, ok := registry.LightByID(index, binding.Light); ok {
			name = light.Name
		}

		lightEvents = append(lightEvents, event.Event{
			Kind: event.LightKind,
			Light: &event.Light{
				LightID: binding.Light,
				Name:    name,
				Action:  binding.Action,
			},
		})
	}

	return lightEvents
}
