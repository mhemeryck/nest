package mqtt

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestTopics(t *testing.T) {
	topics := NewTopics("/nest/", "controller_1")

	assert.Equal(t, "nest", topics.Prefix)
	assert.Equal(t, "controller_1", topics.UnitID)
	assert.Equal(t, "nest/units/controller_1/availability", AvailabilityTopic(topics))
	assert.Equal(t, "nest/units/controller_1/digital_inputs/office_button_input/state", DigitalInputStateTopic(topics, entity.DigitalInputID("office_button_input")))
	assert.Equal(t, "nest/units/controller_1/push_buttons/office_button/state", PushButtonStateTopic(topics, entity.PushButtonID("office_button")))
	assert.Equal(t, "nest/units/controller_1/relays/office_light_relay/state", RelayStateTopic(topics, entity.RelayID("office_light_relay")))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/state", LightStateTopic(topics, entity.LightID("office_light")))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/command", LightCommandTopic(topics, entity.LightID("office_light")))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/state", LightStateTopic(topics, entity.LightID("controller_1.light.office_light")))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/command", LightCommandTopic(topics, entity.LightID("controller_1.light.office_light")))
	assert.Equal(t, "nest/units/controller_1/lights/+/command", LightCommandSubscriptionTopic(topics))
	assert.Equal(t, "nest/units/controller_1/sources/controller_1.button.office_button/event", SemanticSourceEventTopic(topics, entity.ID("controller_1.button.office_button")))
	assert.Equal(t, "nest/units/controller_2/sources/controller_2.button.hall_button/event", SemanticSourceEventTopic(topics, entity.ID("controller_2.button.hall_button")))
	assert.Equal(t, "homeassistant/device/nest_controller_1_unit/config", HomeAssistantDeviceDiscoveryTopic(topics))
}

func TestSemanticSourceEventSubscriptionTopics(t *testing.T) {
	topics := SemanticSourceEventSubscriptionTopics(NewTopics("nest", "controller_1"), &entity.Root{
		RemoteTargetBindings: []entity.Binding{
			{Source: entity.ID("controller_2.button.hall_button"), Target: entity.ID("controller_1.light.office_light"), Action: entity.ActionToggle},
			{Source: entity.ID("controller_2.button.hall_button"), Target: entity.ID("controller_1.light.desk_light"), Action: entity.ActionToggle},
			{Source: entity.ID("controller_3.button.entry_button"), Target: entity.ID("controller_1.light.office_light"), Action: entity.ActionToggle},
		},
	})

	assert.Equal(t, []string{
		"nest/units/controller_2/sources/controller_2.button.hall_button/event",
		"nest/units/controller_3/sources/controller_3.button.entry_button/event",
	}, topics)
}

func TestTopicsWithoutPrefix(t *testing.T) {
	topics := NewTopics("", "controller_1")

	assert.Equal(t, "units/controller_1/availability", AvailabilityTopic(topics))
}

func TestParseLightCommandTopic(t *testing.T) {
	topics := NewTopics("nest", "controller_1")

	lightID, ok := ParseLightCommandTopic(topics, "nest/units/controller_1/lights/office_light/command")
	assert.True(t, ok)
	assert.Equal(t, entity.LightID("controller_1.light.office_light"), lightID)

	_, ok = ParseLightCommandTopic(topics, "nest/units/controller_1/lights/office_light/state")
	assert.False(t, ok)

	_, ok = ParseLightCommandTopic(topics, "nest/units/controller_1/lights/office-light/command")
	assert.False(t, ok)
}
