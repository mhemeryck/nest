package nest

import (
	"context"
	"testing"

	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/require"
)

func TestWaitForShutdownReturnsErrorWhenModbusActorStops(t *testing.T) {
	controllerDone := make(chan struct{})
	sysfsActor := sysfsActor{
		states: make(chan sysfs.StateChange),
		done:   make(chan struct{}),
	}
	mqttActor := mqttActor{
		enabled: true,
		events:  make(chan mqtt.Event),
		done:    make(chan struct{}),
	}
	modbusActor := modbusActor{
		enabled: true,
		events:  make(chan modbus.Event),
		done:    make(chan struct{}),
	}
	close(modbusActor.done)

	err := waitForShutdown(t.Context(), func() {
		close(controllerDone)
		close(sysfsActor.done)
		close(mqttActor.done)
	}, controllerDone, sysfsActor, mqttActor, modbusActor)

	require.ErrorContains(t, err, "modbus actor stopped")
}

func TestWaitForShutdownTreatsCancelledModbusActorAsGraceful(t *testing.T) {
	ctx, cancelContext := context.WithCancel(t.Context())
	cancelContext()
	controllerDone := make(chan struct{})
	sysfsActor := sysfsActor{
		states: make(chan sysfs.StateChange),
		done:   make(chan struct{}),
	}
	mqttActor := mqttActor{
		enabled: true,
		events:  make(chan mqtt.Event),
		done:    make(chan struct{}),
	}
	modbusActor := modbusActor{
		enabled: true,
		events:  make(chan modbus.Event),
		done:    make(chan struct{}),
	}
	close(modbusActor.done)

	err := waitForShutdown(ctx, func() {
		close(controllerDone)
		close(sysfsActor.done)
		close(mqttActor.done)
	}, controllerDone, sysfsActor, mqttActor, modbusActor)

	require.NoError(t, err)
}
