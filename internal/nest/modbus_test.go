package nest

import (
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewModbusActor(t *testing.T) {
	cfg := entity.Modbus{
		Mode:     entity.ModbusModeSlave,
		Port:     "/dev/ttyUSB0",
		BaudRate: 19200,
		Timeout:  time.Second,
		UnitID:   7,
	}

	actor := newModbusActor(registry.Build(&entity.Root{Modbus: cfg}))

	assert.True(t, actor.enabled)
	assert.Equal(t, cfg, actor.cfg)
	require.NotNil(t, actor.commands)
	require.NotNil(t, actor.events)
	require.NotNil(t, actor.done)
}

func TestNewModbusActorDisablesUnconfiguredModbus(t *testing.T) {
	actor := newModbusActor(registry.Build(&entity.Root{}))

	assert.False(t, actor.enabled)
	assert.Nil(t, actor.commands)
	assert.Nil(t, actor.events)
	assert.Nil(t, actor.done)
}

func TestWaitForModbusActorClosesEvents(t *testing.T) {
	actor := modbusActor{
		enabled: true,
		events:  make(chan modbus.Event),
		done:    make(chan struct{}),
	}
	close(actor.done)

	waitForModbusActor(actor)

	_, ok := <-actor.events
	assert.False(t, ok)
}

func TestWaitForModbusActorSkipsDisabledActor(t *testing.T) {
	done := make(chan struct{})
	actor := modbusActor{done: done}

	waitForModbusActor(actor)

	select {
	case <-done:
		require.Fail(t, "disabled actor done channel was closed")
	default:
	}
}
