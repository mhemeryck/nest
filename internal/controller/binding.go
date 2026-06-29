package controller

import (
	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
)

func bindingEventsFromEvent(index *registry.Index, busEvent event.Event) []event.Event {
	switch busEvent.Kind {
	case event.PushButtonPressedKind:
		return lightEventsFromPushButton(index, *busEvent.PushButton)
	default:
		return nil
	}
}

func lightEventsFromPushButton(index *registry.Index, pushButton event.PushButton) []event.Event {
	bindings := registry.BindingsByButton(index, pushButton.ButtonID)
	bindings = append(bindings, registry.RemoteTargetBindingsByButton(index, pushButton.ButtonID)...)
	lightEvents := make([]event.Event, 0, len(bindings))
	for _, binding := range bindings {
		name := ""
		lightID := entity.LightID(binding.Target)
		if light, ok := registry.LightByID(index, lightID); ok {
			name = light.Name
		}

		lightEvents = append(lightEvents, event.Event{
			Kind: event.LightKind,
			Light: &event.Light{
				LightID: lightID,
				Name:    name,
				Action:  entity.LightAction(binding.Action),
			},
		})
	}

	return lightEvents
}
