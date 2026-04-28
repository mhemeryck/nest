package controller

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/event"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushButtonEventsFromStateChange(t *testing.T) {
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), Device: entity.DeviceID("di_3_16")}},
		PushButtons: []entity.PushButton{
			{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")},
			{ID: entity.PushButtonID("office_button_secondary"), Name: "Office button secondary", Input: entity.DigitalInputID("office_button_input")},
		},
	})

	events, handled := pushButtonEventsFromStateChange(index, sysfs.StateChange{
		Device:   sysfs.Device{Identifier: "di_3_16"},
		OldValue: sysfs.Off,
		NewValue: sysfs.On,
		IsRising: true,
	})
	require.True(t, handled)
	require.Len(t, events, 2)

	assert.Equal(t, event.PushButtonKind, events[0].Kind)
	assert.Equal(t, entity.PushButtonID("office_button"), events[0].PushButton.ButtonID)
	assert.Equal(t, "Office button", events[0].PushButton.Name)
	assert.Equal(t, event.PushButtonPressed, events[0].PushButton.Kind)
	assert.Equal(t, entity.PushButtonID("office_button_secondary"), events[1].PushButton.ButtonID)
}

func TestPushButtonEventsFromStateChangeIgnoresUnknownDevices(t *testing.T) {
	index := registry.Build(&entity.Root{})

	_, handled := pushButtonEventsFromStateChange(index, sysfs.StateChange{
		Device: sysfs.Device{Identifier: "ro_3_14"},
	})

	assert.False(t, handled)
}

func TestPushButtonEventsFromStateChangeHandlesFallingEdgeWithoutEvents(t *testing.T) {
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), Device: entity.DeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
	})

	events, handled := pushButtonEventsFromStateChange(index, sysfs.StateChange{
		Device:   sysfs.Device{Identifier: "di_3_16"},
		OldValue: sysfs.On,
		NewValue: sysfs.Off,
		IsRising: false,
	})

	assert.True(t, handled)
	assert.Empty(t, events)
}

func TestLightEventsFromPushButton(t *testing.T) {
	index := registry.Build(&entity.Root{
		Lights:   []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
		Bindings: []entity.Binding{{Button: entity.PushButtonID("office_button"), Light: entity.LightID("office_light"), Action: entity.LightActionToggle}},
	})

	events := lightEventsFromPushButton(index, event.PushButton{ButtonID: entity.PushButtonID("office_button")})
	require.Len(t, events, 1)

	assert.Equal(t, event.LightKind, events[0].Kind)
	assert.Equal(t, entity.LightID("office_light"), events[0].Light.LightID)
	assert.Equal(t, "Office light", events[0].Light.Name)
	assert.Equal(t, entity.LightActionToggle, events[0].Light.Action)
}
