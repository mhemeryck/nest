package controller

import (
	"testing"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverPressAndReleaseBindings(t *testing.T) {
	cover := entity.Cover{ID: "unit.cover.office", Name: "Office"}
	for _, action := range []entity.CoverAction{entity.CoverActionOpen, entity.CoverActionClose} {
		reg := registry.Build(&entity.Root{
			Covers: []entity.Cover{cover}, Lights: []entity.Light{{ID: "light"}},
			Bindings: []entity.Binding{
				{Source: "button", Target: entity.ID(cover.ID), Action: entity.Action(action)},
				{Source: "button", Target: "light", Action: entity.ActionToggle},
			},
		})
		pressed := bindingEventsFromEvent(reg, event.Event{Kind: event.PushButtonPressedKind, PushButton: &event.PushButton{ButtonID: "button"}})
		require.Len(t, pressed, 2)
		assert.Equal(t, event.CoverKind, pressed[0].Kind)
		assert.Equal(t, action, pressed[0].Cover.Action)
		assert.Equal(t, cover.ID, pressed[0].Cover.CoverID)
		assert.Equal(t, entity.LightActionToggle, pressed[1].Light.Action)
		released := bindingEventsFromEvent(reg, event.Event{Kind: event.PushButtonReleasedKind, PushButton: &event.PushButton{ButtonID: "button"}})
		require.Len(t, released, 1)
		assert.Equal(t, entity.CoverActionStop, released[0].Cover.Action)
	}
}

func TestRemoteCoverReleaseUsesDeliveryTransport(t *testing.T) {
	reg := registry.Build(&entity.Root{
		Covers: []entity.Cover{{ID: "unit.cover.office"}},
		RemoteTargetBindings: []entity.Binding{{
			Source: "remote.button.open", Target: "unit.cover.office", Action: "open", ExecutionTransport: entity.ExecutionTransportMQTT,
		}},
	})
	button := &event.PushButton{ButtonID: "remote.button.open", Delivery: entity.ExecutionTransportMQTT}
	events := bindingEventsFromEvent(reg, event.Event{Kind: event.PushButtonReleasedKind, PushButton: button})
	require.Len(t, events, 1)
	assert.Equal(t, entity.CoverActionStop, events[0].Cover.Action)
	button.Delivery = entity.ExecutionTransportModbus
	assert.Empty(t, bindingEventsFromEvent(reg, event.Event{Kind: event.PushButtonReleasedKind, PushButton: button}))
}
