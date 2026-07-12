package mqtt

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartupCommands(t *testing.T) {
	cfg := entity.MQTT{
		TopicPrefix: "nest",
		UnitID:      "controller_1",
	}
	lights := []entity.Light{{
		ID:    entity.LightID("office_light"),
		Name:  "Office light",
		Relay: entity.RelayID("office_light_relay"),
	}}

	commands, err := StartupCommands(cfg, lights)
	require.NoError(t, err)
	require.Len(t, commands, 2)

	assert.Equal(t, PublishCommandKind, commands[0].Kind)
	assert.Equal(t, "homeassistant/device/nest_controller_1_unit/config", commands[0].Publish.Topic)
	assert.True(t, commands[0].Publish.Retain)
	assert.Contains(t, string(commands[0].Publish.Payload), `"cmps"`)
	assert.Contains(t, string(commands[0].Publish.Payload), `"unique_id":"nest_controller_1_office_light"`)
	assert.Contains(t, string(commands[0].Publish.Payload), `"default_entity_id":"light.controller_1_office_light"`)
	assert.Contains(t, string(commands[0].Publish.Payload), `"command_topic":"nest/units/controller_1/lights/office_light/command"`)
	assert.Equal(t, PublishCommandKind, commands[1].Kind)
	assert.Equal(t, "nest/units/controller_1/availability", commands[1].Publish.Topic)
	assert.Equal(t, []byte("online"), commands[1].Publish.Payload)
	assert.True(t, commands[1].Publish.Retain)
}
