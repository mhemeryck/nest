package controller

import (
	"testing"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushButtonEventsFromStateChange(t *testing.T) {
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")}},
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

	assert.Equal(t, event.PushButtonPressedKind, events[0].Kind)
	assert.Equal(t, entity.PushButtonID("office_button"), events[0].PushButton.ButtonID)
	assert.Equal(t, "Office button", events[0].PushButton.Name)
	assert.Equal(t, entity.PushButtonID("office_button_secondary"), events[1].PushButton.ButtonID)
}

func TestPushButtonEventsFromStateChangeIgnoresUnknownDevices(t *testing.T) {
	index := registry.Build(&entity.Root{})

	_, handled := pushButtonEventsFromStateChange(index, sysfs.StateChange{
		Device: sysfs.Device{Identifier: "ro_3_14"},
	})

	assert.False(t, handled)
}

func TestPushButtonEventsFromStateChangeMapsFallingEdgeToRelease(t *testing.T) {
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), SysfsDevice: entity.SysfsDeviceID("di_3_16")}},
		PushButtons:   []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
	})

	events, handled := pushButtonEventsFromStateChange(index, sysfs.StateChange{
		Device:   sysfs.Device{Identifier: "di_3_16"},
		OldValue: sysfs.On,
		NewValue: sysfs.Off,
		IsRising: false,
	})

	require.True(t, handled)
	require.Len(t, events, 1)
	assert.Equal(t, event.PushButtonReleasedKind, events[0].Kind)
	assert.Equal(t, entity.PushButtonID("office_button"), events[0].PushButton.ButtonID)
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

func TestSemanticEventsFromRelayStateChangeIncludesLightState(t *testing.T) {
	index := registry.Build(&entity.Root{
		Relays: []entity.Relay{{ID: entity.RelayID("office_light_relay"), Name: "Office light relay", SysfsDevice: entity.SysfsDeviceID("ro_3_14")}},
		Lights: []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
	})

	events, handled := semanticEventsFromStateChange(index, sysfs.StateChange{
		Device:   sysfs.Device{Identifier: "ro_3_14"},
		OldValue: sysfs.Off,
		NewValue: sysfs.On,
		IsRising: true,
	})

	require.True(t, handled)
	require.Len(t, events, 2)
	assert.Equal(t, event.RelayStateKind, events[0].Kind)
	assert.Equal(t, entity.RelayID("office_light_relay"), events[0].Relay.RelayID)
	assert.Equal(t, 1, events[0].Relay.Value)
	assert.Equal(t, event.LightStateKind, events[1].Kind)
	assert.Equal(t, entity.LightID("office_light"), events[1].LightState.LightID)
	assert.Equal(t, entity.RelayID("office_light_relay"), events[1].LightState.RelayID)
	assert.Equal(t, 1, events[1].LightState.Value)
}

func TestSemanticEventFromMQTTEventMapsLightCommandToLightEvent(t *testing.T) {
	root := &entity.Root{
		Lights: []entity.Light{{ID: entity.LightID("controller_1.light.office_light"), Name: "Office light", Relay: entity.RelayID("office_light_relay")}},
	}

	semanticEvent, handled := semanticEventFromMQTTEvent(
		registry.Build(root),
		mqtt.NewTopics("nest", "controller_1"),
		mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: "nest/units/controller_1/lights/office_light/command", Payload: []byte(" on\n")}),
	)

	require.True(t, handled)
	assert.Equal(t, event.LightKind, semanticEvent.Kind)
	assert.Equal(t, entity.LightID("controller_1.light.office_light"), semanticEvent.Light.LightID)
	assert.Equal(t, entity.LightActionOn, semanticEvent.Light.Action)
}

func TestSemanticEventFromMQTTEventRejectsInvalidLightCommandPayload(t *testing.T) {
	semanticEvent, handled := semanticEventFromMQTTEvent(
		registry.Build(&entity.Root{Lights: []entity.Light{{ID: entity.LightID("office_light"), Name: "Office light"}}}),
		mqtt.NewTopics("nest", "controller_1"),
		mqtt.ReceivedEvent(mqtt.ReceivedMessage{Topic: "nest/units/controller_1/lights/office_light/command", Payload: []byte("TOGGLE")}),
	)

	assert.False(t, handled)
	assert.Equal(t, event.Event{}, semanticEvent)
}
