package mqtt

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartupCommands(t *testing.T) {
	root := &entity.Root{
		MQTT: entity.MQTT{
			TopicPrefix: "nest",
			UnitID:      "controller_1",
		},
		Relays: []entity.Relay{{
			ID:          entity.RelayID("office_light_relay"),
			Name:        "Office light relay",
			SysfsDevice: entity.SysfsDeviceID("ro_3_14"),
		}},
		Lights: []entity.Light{{
			ID:    entity.LightID("office_light"),
			Name:  "Office light",
			Relay: entity.RelayID("office_light_relay"),
		}},
	}

	commands, err := StartupCommands(root)
	require.NoError(t, err)
	require.Len(t, commands, 2)

	assert.Equal(t, PublishCommandKind, commands[0].Kind)
	assert.Equal(t, "nest/units/controller_1/discovery", commands[0].Publish.Topic)
	assert.True(t, commands[0].Publish.Retain)
	assert.Contains(t, string(commands[0].Publish.Payload), `"commands_enabled":false`)
	assert.Equal(t, PublishCommandKind, commands[1].Kind)
	assert.Equal(t, "nest/units/controller_1/availability", commands[1].Publish.Topic)
	assert.Equal(t, []byte("online"), commands[1].Publish.Payload)
	assert.True(t, commands[1].Publish.Retain)
}
