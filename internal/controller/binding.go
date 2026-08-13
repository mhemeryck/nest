package controller

import (
	"slices"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
)

func bindingEventsFromEvent(index *registry.Registry, busEvent event.Event) []event.Event {
	switch busEvent.Kind {
	case event.PushButtonPressedKind:
		return lightEventsFromPushButton(index, *busEvent.PushButton)
	default:
		return nil
	}
}

func lightEventsFromPushButton(index *registry.Registry, pushButton event.PushButton) []event.Event {
	bindings := registry.BindingsByButton(index, pushButton.ButtonID)
	bindings = append(bindings, remoteTargetBindingsBySourceAndTransport(index, entity.ID(pushButton.ButtonID), pushButton.Delivery)...)
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

func remoteTargetBindingsBySourceAndTransport(index *registry.Registry, sourceID entity.ID, delivery entity.ExecutionTransport) []entity.Binding {
	bindings := registry.RemoteTargetBindingsBySource(index, sourceID)
	return slices.DeleteFunc(bindings, func(binding entity.Binding) bool {
		return binding.ExecutionTransport != delivery
	})
}
