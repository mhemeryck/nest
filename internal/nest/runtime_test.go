package nest

import (
	"context"
	"testing"
	"time"

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

func TestActorFailureRequestsControllerShutdownBeforeCancellingActors(t *testing.T) {
	ctx, cancelController := context.WithCancel(t.Context())
	actorContext, cancelActors := context.WithCancel(t.Context())
	defer cancelActors()
	controllerDone := make(chan struct{})
	sysfsActor := sysfsActor{states: make(chan sysfs.StateChange), done: make(chan struct{})}
	close(sysfsActor.done)
	feedbackSurvived := make(chan bool, 1)
	go func() {
		<-ctx.Done()
		feedbackSurvived <- actorContext.Err() == nil
		close(controllerDone)
	}()
	err := waitForShutdown(ctx, cancelActors, controllerDone, sysfsActor, mqttActor{}, modbusActor{}, shutdownOptions{cancelController: cancelController, period: time.Second})
	require.ErrorContains(t, err, "sysfs actor stopped")
	require.True(t, <-feedbackSurvived)
	require.ErrorIs(t, actorContext.Err(), context.Canceled)
}

func TestBlockedActorShutdownDoesNotDeadlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	actor := sysfsActor{states: make(chan sysfs.StateChange, 1), done: make(chan struct{})}
	require.False(t, waitForActorsWithin(ctx, actor, mqttActor{}, modbusActor{}, persistenceActor{}))
	// A late writer must not encounter a channel closed before worker exit
	actor.states <- sysfs.StateChange{}
}

func TestMQTTActorFailureDoesNotStopLocalControl(t *testing.T) {
	ctx, cancelController := context.WithCancel(t.Context())
	actorContext, cancelActors := context.WithCancel(t.Context())
	controllerDone := make(chan struct{})
	sysfsActor := sysfsActor{states: make(chan sysfs.StateChange), done: make(chan struct{})}
	mqttActor := mqttActor{enabled: true, events: make(chan mqtt.Event), done: make(chan struct{})}
	close(mqttActor.done)
	go func() { <-ctx.Done(); close(controllerDone) }()
	result := make(chan error, 1)
	go func() {
		result <- waitForShutdown(ctx, func() { cancelActors(); close(sysfsActor.done) }, controllerDone, sysfsActor, mqttActor, modbusActor{}, shutdownOptions{cancelController: cancelController, period: time.Second})
	}()
	defer func() { cancelController(); cancelActors() }()
	select {
	case <-result:
		require.FailNow(t, "MQTT termination stopped the runtime")
	case <-time.After(20 * time.Millisecond):
	}
	require.NoError(t, ctx.Err())
	require.NoError(t, actorContext.Err())
	cancelController()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "runtime failed to stop after user shutdown")
	}
}
