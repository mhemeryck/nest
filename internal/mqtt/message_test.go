package mqtt

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAvailabilityMessage(t *testing.T) {
	message := AvailabilityMessage(NewTopics("nest", "controller_1"), AvailabilityOnline)

	assert.Equal(t, "nest/units/controller_1/availability", message.Topic)
	assert.Equal(t, []byte("online"), message.Payload)
	assert.True(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}

func TestDiscoveryMessage(t *testing.T) {
	message, err := DiscoveryMessage(testEntityRoot(), NewTopics("nest", "controller_1"))
	require.NoError(t, err)

	assert.Equal(t, "nest/units/controller_1/discovery", message.Topic)
	assert.Contains(t, string(message.Payload), `"commands_enabled":false`)
	assert.True(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}

func TestDigitalInputStateMessage(t *testing.T) {
	message, err := DigitalInputStateMessage(NewTopics("nest", "controller_1"), DigitalInputObservation{
		InputID:     entity.DigitalInputID("office_button_input"),
		SysfsDevice: entity.SysfsDeviceID("di_3_16"),
		Value:       1,
	})
	require.NoError(t, err)

	assert.Equal(t, "nest/units/controller_1/digital_inputs/office_button_input/state", message.Topic)
	assert.JSONEq(t, `{"input_id":"office_button_input","sysfs_device":"di_3_16","value":1}`, string(message.Payload))
	assert.True(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}

func TestPushButtonStateMessage(t *testing.T) {
	message, err := PushButtonStateMessage(NewTopics("nest", "controller_1"), PushButtonObservation{
		ButtonID: entity.PushButtonID("office_button"),
		Name:     "Office button",
		State:    "pressed",
	})
	require.NoError(t, err)

	assert.Equal(t, "nest/units/controller_1/push_buttons/office_button/state", message.Topic)
	assert.JSONEq(t, `{"button_id":"office_button","name":"Office button","state":"pressed"}`, string(message.Payload))
	assert.False(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}

func TestRelayStateMessage(t *testing.T) {
	message, err := RelayStateMessage(NewTopics("nest", "controller_1"), RelayObservation{
		RelayID:     entity.RelayID("office_light_relay"),
		Name:        "Office light relay",
		SysfsDevice: entity.SysfsDeviceID("ro_3_14"),
		Value:       1,
	})
	require.NoError(t, err)

	assert.Equal(t, "nest/units/controller_1/relays/office_light_relay/state", message.Topic)
	assert.JSONEq(t, `{"relay_id":"office_light_relay","name":"Office light relay","sysfs_device":"ro_3_14","value":1}`, string(message.Payload))
	assert.True(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}

func TestLightStateMessage(t *testing.T) {
	message, err := LightStateMessage(NewTopics("nest", "controller_1"), LightObservation{
		LightID: entity.LightID("office_light"),
		Name:    "Office light",
		RelayID: entity.RelayID("office_light_relay"),
		Value:   1,
	})
	require.NoError(t, err)

	assert.Equal(t, "nest/units/controller_1/lights/office_light/state", message.Topic)
	assert.JSONEq(t, `{"light_id":"office_light","name":"Office light","relay_id":"office_light_relay","value":1}`, string(message.Payload))
	assert.True(t, message.Retain)
	assert.Equal(t, byte(0), message.QoS)
}
