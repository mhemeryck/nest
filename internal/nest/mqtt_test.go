package nest

import (
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
)

func TestNewMQTTActorBuffersInitialLocalLightStates(t *testing.T) {
	lights := make([]entity.Light, 40)
	actor := newMQTTActor(registry.Build(&entity.Root{
		MQTT:   entity.MQTT{Enabled: true},
		Lights: lights,
	}))

	assert.Equal(t, 32+len(lights), cap(actor.commands))
}
