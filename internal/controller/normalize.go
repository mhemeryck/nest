package controller

import (
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/event"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

func pushButtonEventsFromStateChange(index *registry.Index, stateChange sysfs.StateChange) ([]event.Event, bool) {
	input, ok := index.DigitalInputsByDevice[entity.DeviceID(stateChange.Device.Identifier)]
	if !ok {
		return nil, false
	}

	if !stateChange.IsRising {
		return nil, true
	}

	buttons := index.PushButtonsByInputID[input.ID]
	buttonEvents := make([]event.Event, 0, len(buttons))
	for _, button := range buttons {
		buttonEvents = append(buttonEvents, event.Event{
			Kind: event.PushButtonKind,
			PushButton: &event.PushButton{
				ButtonID: button.ID,
				Name:     button.Name,
				Kind:     event.PushButtonPressed,
			},
		})
	}

	return buttonEvents, true
}

func lightEventsFromPushButton(index *registry.Index, pushButton event.PushButton) []event.Event {
	bindings := index.BindingsByButtonID[pushButton.ButtonID]
	lightEvents := make([]event.Event, 0, len(bindings))
	for _, binding := range bindings {
		name := ""
		if light, ok := index.LightsByID[binding.Light]; ok {
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
