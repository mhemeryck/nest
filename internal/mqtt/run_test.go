package mqtt

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestBrokerURL(t *testing.T) {
	url := brokerURL(entity.MQTT{Host: "mqtt.local", Port: 1883})

	assert.Equal(t, "tcp://mqtt.local:1883", url)
}

func TestClientOptions(t *testing.T) {
	options := clientOptions(entity.MQTT{
		Host:        "mqtt.local",
		Port:        1883,
		ClientID:    "nest-controller-1",
		Username:    "nest",
		Password:    "secret",
		TopicPrefix: "nest",
		UnitID:      "controller_1",
	})

	assert.Equal(t, "nest-controller-1", options.ClientID)
	assert.Equal(t, "nest", options.Username)
	assert.Equal(t, "secret", options.Password)
	assert.True(t, options.WillEnabled)
	assert.Equal(t, "nest/units/controller_1/availability", options.WillTopic)
	assert.Equal(t, []byte("offline"), options.WillPayload)
	assert.True(t, options.WillRetained)
	assert.Equal(t, byte(0), options.WillQos)
	if assert.Len(t, options.Servers, 1) {
		assert.Equal(t, "tcp://mqtt.local:1883", options.Servers[0].String())
	}
}
