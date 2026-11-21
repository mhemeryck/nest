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
	assert.Equal(t, "nest/units/controller_1/digital_inputs/office_button_input/state", digitalInputTopic(topics, entity.DigitalInputID("office_button_input"), stateSuffix))
	assert.Equal(t, "nest/units/controller_1/push_buttons/office_button/state", pushButtonTopic(topics, entity.PushButtonID("office_button"), stateSuffix))
	assert.Equal(t, "nest/units/controller_1/relays/office_light_relay/state", relayTopic(topics, entity.RelayID("office_light_relay"), stateSuffix))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/state", lightTopic(topics, entity.LightID("office_light"), stateSuffix))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/command", lightTopic(topics, entity.LightID("office_light"), commandSuffix))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/state", lightTopic(topics, entity.LightID("controller_1.light.office_light"), stateSuffix))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/command", lightTopic(topics, entity.LightID("controller_1.light.office_light"), commandSuffix))
	assert.Equal(t, "nest/units/controller_1/lights/+/command", LightCommandSubscriptionTopic(topics))
	assert.Equal(t, "nest/units/controller_1/sources/controller_1.button.office_button/event", SemanticSourceEventTopic(topics, entity.ID("controller_1.button.office_button")))
	assert.Equal(t, "nest/units/controller_2/sources/controller_2.button.hall_button/event", SemanticSourceEventTopic(topics, entity.ID("controller_2.button.hall_button")))
	assert.Equal(t, "homeassistant/device/nest_controller_1_unit/config", HomeAssistantDeviceDiscoveryTopic(topics))
}

func TestSemanticSourceEventSubscriptionTopics(t *testing.T) {
	topics := SemanticSourceEventSubscriptionTopics(NewTopics("nest", "controller_1"), []entity.Binding{
		{Source: entity.ID("controller_2.button.hall_button"), Target: entity.ID("controller_1.light.office_light"), Action: entity.ActionToggle},
		{Source: entity.ID("controller_2.button.hall_button"), Target: entity.ID("controller_1.light.desk_light"), Action: entity.ActionToggle},
		{Source: entity.ID("controller_3.button.entry_button"), Target: entity.ID("controller_1.light.office_light"), Action: entity.ActionToggle},
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

func TestParseEntityCommandTopic(t *testing.T) {
	topics := NewTopics("nest", "controller_1")

	kind, id := ParseEntityCommandTopic(topics, "nest/units/controller_1/lights/office_light/command")
	assert.Equal(t, entity.TypeLight, kind)
	assert.Equal(t, entity.ID("controller_1.light.office_light"), id)

	kind, id = ParseEntityCommandTopic(topics, "nest/units/controller_1/covers/office/command")
	assert.Equal(t, entity.TypeCover, kind)
	assert.Equal(t, entity.ID("controller_1.cover.office"), id)

	for _, topic := range []string{
		"nest/units/controller_1/lights/office_light/state",
		"nest/units/controller_1/lights/office-light/command",
		"nest/units/controller_1/covers/office/extra/command",
		"nest/units/controller_2/covers/office/command",
		"nest/units/controller_1/relays/office/command",
	} {
		t.Run(topic, func(t *testing.T) {
			kind, id := ParseEntityCommandTopic(topics, topic)
			assert.Empty(t, kind)
			assert.Empty(t, id)
		})
	}
}
