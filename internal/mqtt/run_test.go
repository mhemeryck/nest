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
		Host:     "mqtt.local",
		Port:     1883,
		ClientID: "nest-controller-1",
		Username: "nest",
		Password: "secret",
	})

	assert.Equal(t, "nest-controller-1", options.ClientID)
	assert.Equal(t, "nest", options.Username)
	assert.Equal(t, "secret", options.Password)
	if assert.Len(t, options.Servers, 1) {
		assert.Equal(t, "tcp://mqtt.local:1883", options.Servers[0].String())
	}
}
