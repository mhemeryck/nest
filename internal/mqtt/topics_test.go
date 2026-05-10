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
	assert.Equal(t, "nest/units/controller_1/discovery", DiscoveryTopic(topics))
	assert.Equal(t, "nest/units/controller_1/digital_inputs/office_button_input/state", DigitalInputStateTopic(topics, entity.DigitalInputID("office_button_input")))
	assert.Equal(t, "nest/units/controller_1/push_buttons/office_button/state", PushButtonStateTopic(topics, entity.PushButtonID("office_button")))
	assert.Equal(t, "nest/units/controller_1/relays/office_light_relay/state", RelayStateTopic(topics, entity.RelayID("office_light_relay")))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/state", LightStateTopic(topics, entity.LightID("office_light")))
	assert.Equal(t, "nest/units/controller_1/lights/office_light/command", LightCommandTopic(topics, entity.LightID("office_light")))
}

func TestTopicsWithoutPrefix(t *testing.T) {
	topics := NewTopics("", "controller_1")

	assert.Equal(t, "units/controller_1/availability", AvailabilityTopic(topics))
}
