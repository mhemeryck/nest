package mqtt

import (
	"testing"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/assert"
)

func TestSysfsStateChange(t *testing.T) {
	state := SysfsStateChange(Config{UnitID: "tesla"}, sysfs.StateChange{
		Device:   sysfs.Device{Type: sysfs.DigitalInput, Identifier: "di_3_16"},
		OldValue: sysfs.Off,
		NewValue: sysfs.On,
		IsRising: true,
	})

	assert.Equal(t, SysfsState, state.Kind)
	assert.Equal(t, "tesla/sysfs/di_3_16/state", state.Topic)
	assert.True(t, state.Retain)
	assert.JSONEq(t, `{"device_id":"di_3_16","type":"digital_input","old":"OFF","new":"ON","rising":true,"falling":false}`, string(state.Payload))
}

func TestDigitalInputStates(t *testing.T) {
	states := DigitalInputStates(Config{UnitID: "tesla"}, event.DigitalInputEvent{
		InputID:  entity.DigitalInputID("office_button_input"),
		DeviceID: entity.DeviceID("di_3_16"),
		IsRising: true,
	}, sysfs.On)

	assert.Len(t, states, 2)
	assert.Equal(t, "tesla/input/3_16/state", states[0].Topic)
	assert.Equal(t, "ON", string(states[0].Payload))
	assert.True(t, states[0].Retain)
	assert.Equal(t, "tesla/input/3_16/event", states[1].Topic)
	assert.JSONEq(t, `{"input_id":"office_button_input","device_id":"di_3_16","state":"ON","rising":true,"falling":false}`, string(states[1].Payload))
}

func TestRelayStateChange(t *testing.T) {
	state := RelayStateChange(Config{UnitID: "edison"}, entity.Relay{Device: entity.DeviceID("ro_3_14")}, sysfs.Off)

	assert.Equal(t, RelayState, state.Kind)
	assert.Equal(t, "edison/relay/3_14/state", state.Topic)
	assert.Equal(t, "OFF", string(state.Payload))
	assert.True(t, state.Retain)
}
