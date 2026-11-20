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
		return eventsFromPushButton(index, *busEvent.PushButton, busEvent.Kind)
	default:
		return nil
	}
}

func eventsFromPushButton(index *registry.Registry, pushButton event.PushButton, kind event.Kind) []event.Event {
	bindings := registry.BindingsByButton(index, pushButton.ButtonID)
	if kind == event.PushButtonPressedKind {
		bindings = append(bindings, remoteTargetBindingsBySourceAndTransport(index, entity.ID(pushButton.ButtonID), pushButton.Delivery)...)
	}
	events := make([]event.Event, 0, len(bindings))
	for _, binding := range bindings {
		switch binding.Action {
		case entity.ActionToggle:
			events = appendLightBindingEvent(events, index, binding, kind)
		case entity.Action(entity.CoverActionOpen), entity.Action(entity.CoverActionClose):
			events = append(events, coverEventFromBinding(binding, kind))
		}
	}
	return events
}

func appendLightBindingEvent(events []event.Event, index *registry.Registry, binding entity.Binding, kind event.Kind) []event.Event {
	switch kind {
	case event.PushButtonPressedKind:
		lightID := entity.LightID(binding.Target)
		light, _ := registry.LightByID(index, lightID)
		return append(events, event.Event{
			Kind: event.LightKind,
			Light: &event.Light{
				LightID: lightID,
				Name:    light.Name,
				Action:  entity.LightAction(binding.Action),
			},
		})
	case event.PushButtonReleasedKind:
		return events
	default:
		panic("light binding requires a push button press or release")
	}
}

func coverEventFromBinding(binding entity.Binding, kind event.Kind) event.Event {
	var action entity.CoverAction
	switch kind {
	case event.PushButtonPressedKind:
		action = entity.CoverAction(binding.Action)
	case event.PushButtonReleasedKind:
		action = entity.CoverActionStop
	default:
		panic("cover binding requires a push button press or release")
	}
	return event.Event{
		Kind:  event.CoverKind,
		Cover: &event.Cover{CoverID: entity.CoverID(binding.Target), Action: action},
	}
}

func remoteTargetBindingsBySourceAndTransport(index *registry.Registry, sourceID entity.ID, delivery entity.ExecutionTransport) []entity.Binding {
	bindings := registry.RemoteTargetBindingsBySource(index, sourceID)
	return slices.DeleteFunc(bindings, func(binding entity.Binding) bool {
		return binding.ExecutionTransport != delivery
	})
}
