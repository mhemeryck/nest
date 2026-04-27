package event

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateChangeToDigitalInputEvent(t *testing.T) {
	index := registry.Build(&entity.Root{
		DigitalInputs: []entity.DigitalInput{{ID: entity.DigitalInputID("office_button_input"), Device: entity.DeviceID("di_3_16")}},
	})

	event, ok := StateChangeToDigitalInputEvent(index, sysfs.StateChange{
		Device:   sysfs.Device{Identifier: "di_3_16"},
		OldValue: sysfs.Off,
		NewValue: sysfs.On,
		IsRising: true,
	})
	require.True(t, ok)
	require.NotNil(t, event.DigitalInput)

	assert.Equal(t, DigitalInputKind, event.Kind)
	assert.Equal(t, entity.DigitalInputID("office_button_input"), event.DigitalInput.InputID)
	assert.Equal(t, entity.DeviceID("di_3_16"), event.DigitalInput.DeviceID)
	assert.True(t, event.DigitalInput.IsRising)
	assert.False(t, event.DigitalInput.IsFalling)
}

func TestStateChangeToDigitalInputEventIgnoresUnknownDevices(t *testing.T) {
	index := registry.Build(&entity.Root{})

	_, ok := StateChangeToDigitalInputEvent(index, sysfs.StateChange{
		Device: sysfs.Device{Identifier: "ro_3_14"},
	})
	assert.False(t, ok)
}

func TestDigitalInputEventToPushButtonEvents(t *testing.T) {
	index := registry.Build(&entity.Root{
		PushButtons: []entity.PushButton{
			{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")},
			{ID: entity.PushButtonID("office_button_secondary"), Name: "Office button secondary", Input: entity.DigitalInputID("office_button_input")},
		},
	})

	events := DigitalInputEventToPushButtonEvents(index, DigitalInputEvent{
		InputID:  entity.DigitalInputID("office_button_input"),
		IsRising: true,
	})
	require.Len(t, events, 2)

	assert.Equal(t, PushButtonKind, events[0].Kind)
	assert.Equal(t, entity.PushButtonID("office_button"), events[0].PushButton.ButtonID)
	assert.Equal(t, PushButtonPressed, events[0].PushButton.Kind)
	assert.Equal(t, entity.PushButtonID("office_button_secondary"), events[1].PushButton.ButtonID)
}

func TestDigitalInputEventToPushButtonEventsIgnoresFallingEdge(t *testing.T) {
	index := registry.Build(&entity.Root{
		PushButtons: []entity.PushButton{{ID: entity.PushButtonID("office_button"), Name: "Office button", Input: entity.DigitalInputID("office_button_input")}},
	})

	events := DigitalInputEventToPushButtonEvents(index, DigitalInputEvent{
		InputID:   entity.DigitalInputID("office_button_input"),
		IsFalling: true,
		IsRising:  false,
	})
	assert.Empty(t, events)
}
