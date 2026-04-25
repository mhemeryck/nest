package mqtt

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestDeviceTopicSlot(t *testing.T) {
	assert.Equal(t, "3_16", DeviceTopicSlot(entity.DeviceID("di_3_16")))
	assert.Equal(t, "3_14", DeviceTopicSlot(entity.DeviceID("ro_3_14")))
	assert.Equal(t, "custom", DeviceTopicSlot(entity.DeviceID("custom")))
}

func TestTopics(t *testing.T) {
	cfg := Config{UnitID: "tesla"}

	assert.Equal(t, "tesla/availability", AvailabilityTopic(cfg))
	assert.Equal(t, "tesla/input/3_16/state", InputStateTopic(cfg, entity.DeviceID("di_3_16")))
	assert.Equal(t, "tesla/input/3_16/set", InputCommandTopic(cfg, entity.DeviceID("di_3_16")))
	assert.Equal(t, "tesla/relay/3_14/state", RelayStateTopic(cfg, entity.DeviceID("ro_3_14")))
	assert.Equal(t, "tesla/relay/3_14/set", RelayCommandTopic(cfg, entity.DeviceID("ro_3_14")))
	assert.Equal(t, "homeassistant/switch/tesla_office_button/config", HASwitchDiscoveryTopic(cfg, entity.PushButtonID("office_button")))
	assert.Equal(t, "homeassistant/light/tesla_office_light/config", HALightDiscoveryTopic(cfg, entity.LightID("office_light")))
}
