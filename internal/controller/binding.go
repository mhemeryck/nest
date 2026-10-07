package controller

import (
	"slices"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
)

func bindingEventsFromEvent(index *registry.Registry, busEvent event.Event) []event.Event {
	switch busEvent.Kind {
	case event.PushButtonPressedKind, event.PushButtonReleasedKind:
		return bindingEventsFromPushButton(index, busEvent.Kind, *busEvent.PushButton)
	default:
		return nil
	}
}

func bindingEventsFromPushButton(index *registry.Registry, kind event.Kind, pushButton event.PushButton) []event.Event {
	bindings := registry.BindingsByButton(index, pushButton.ButtonID)
	bindings = append(bindings, remoteTargetBindingsBySourceAndTransport(index, entity.ID(pushButton.ButtonID), pushButton.Delivery)...)
	derivedEvents := make([]event.Event, 0, len(bindings))
	for _, binding := range bindings {
		if cover, ok := registry.CoverByID(index, entity.CoverID(binding.Target)); ok {
			action := entity.CoverAction(binding.Action)
			if kind == event.PushButtonReleasedKind {
				action = entity.CoverActionStop
			}
			derivedEvents = append(derivedEvents, event.Event{Kind: event.CoverKind, Cover: &event.Cover{
				CoverID: cover.ID, Name: cover.Name, Action: action,
			}})
			continue
		}
		if kind == event.PushButtonReleasedKind {
			continue
		}
		name := ""
		lightID := entity.LightID(binding.Target)
		if light, ok := registry.LightByID(index, lightID); ok {
			name = light.Name
		}

		derivedEvents = append(derivedEvents, event.Event{
			Kind: event.LightKind,
			Light: &event.Light{
				LightID: lightID,
				Name:    name,
				Action:  entity.LightAction(binding.Action),
			},
		})
	}

	return derivedEvents
}

func remoteTargetBindingsBySourceAndTransport(index *registry.Registry, sourceID entity.ID, delivery entity.ExecutionTransport) []entity.Binding {
	bindings := registry.RemoteTargetBindingsBySource(index, sourceID)
	return slices.DeleteFunc(bindings, func(binding entity.Binding) bool {
		return binding.ExecutionTransport != delivery
	})
}
