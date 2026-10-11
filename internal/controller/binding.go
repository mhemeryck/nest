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
		if derived, handled := eventFromButtonBinding(index, kind, binding); handled {
			derivedEvents = append(derivedEvents, derived)
		}
	}

	return derivedEvents
}

func eventFromButtonBinding(index *registry.Registry, kind event.Kind, binding entity.Binding) (event.Event, bool) {
	if cover, ok := registry.CoverByID(index, entity.CoverID(binding.Target)); ok {
		return coverEventFromButtonBinding(cover, kind, binding), true
	}
	if kind == event.PushButtonReleasedKind {
		return event.Event{}, false
	}
	return lightEventFromButtonBinding(index, binding), true
}

func coverEventFromButtonBinding(cover entity.Cover, kind event.Kind, binding entity.Binding) event.Event {
	action := entity.CoverAction(binding.Action)
	if kind == event.PushButtonReleasedKind {
		action = entity.CoverActionStop
	}
	return event.Event{Kind: event.CoverKind, Cover: &event.Cover{
		CoverID: cover.ID, Name: cover.Name, Action: action,
	}}
}

func lightEventFromButtonBinding(index *registry.Registry, binding entity.Binding) event.Event {
	name := ""
	lightID := entity.LightID(binding.Target)
	if light, ok := registry.LightByID(index, lightID); ok {
		name = light.Name
	}
	return event.Event{Kind: event.LightKind, Light: &event.Light{
		LightID: lightID, Name: name, Action: entity.LightAction(binding.Action),
	}}
}

func remoteTargetBindingsBySourceAndTransport(index *registry.Registry, sourceID entity.ID, delivery entity.ExecutionTransport) []entity.Binding {
	bindings := registry.RemoteTargetBindingsBySource(index, sourceID)
	return slices.DeleteFunc(bindings, func(binding entity.Binding) bool {
		return binding.ExecutionTransport != delivery
	})
}
