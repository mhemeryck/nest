package nest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/controller"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
	"github.com/stretchr/testify/require"
)

func TestWaitForShutdownReturnsErrorWhenModbusActorStops(t *testing.T) {
	controllerDone := make(chan error, 1)
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
	}, controllerDone, sysfsActor, mqttActor, modbusActor, shutdownOptions{})

	require.ErrorContains(t, err, "modbus actor stopped")
}

func TestWaitForShutdownTreatsCancelledModbusActorAsGraceful(t *testing.T) {
	ctx, cancelContext := context.WithCancel(t.Context())
	cancelContext()
	controllerDone := make(chan error, 1)
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
	}, controllerDone, sysfsActor, mqttActor, modbusActor, shutdownOptions{})

	require.NoError(t, err)
}

func TestActorFailureRequestsControllerShutdownBeforeCancellingActors(t *testing.T) {
	ctx, cancelController := context.WithCancel(t.Context())
	actorContext, cancelActors := context.WithCancel(t.Context())
	defer cancelActors()
	controllerDone := make(chan error, 1)
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
	controllerDone := make(chan error, 1)
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

func TestFailedCoverShutdownRemainsErrorAfterActorTermination(t *testing.T) {
	ctx, cancelController := context.WithCancel(t.Context())
	defer cancelController()
	actorContext, cancelActors := context.WithCancel(t.Context())
	defer cancelActors()
	reg := registry.Build(&entity.Root{
		CoverControl: entity.CoverControl{OperationTimeout: time.Second, OffRetryInterval: 5 * time.Millisecond, ShutdownPeriod: 40 * time.Millisecond},
		Covers:       []entity.Cover{{ID: "unit.cover.office", OpenRelay: "open", CloseRelay: "close"}},
		Relays:       []entity.Relay{{ID: "open", SysfsDevice: "ro_1_1"}, {ID: "close", SysfsDevice: "ro_1_2"}},
	})
	actor := sysfsActor{commands: make(chan sysfs.Command, 32), states: make(chan sysfs.StateChange, 32), done: make(chan struct{})}
	go func() {
		defer close(actor.done)
		for {
			select {
			case <-actorContext.Done():
				return
			case command := <-actor.commands:
				select {
				case actor.states <- sysfs.StateChange{Kind: sysfs.CompletionReportKind, Completion: &sysfs.CommandCompletion{
					Command: command, CompletedAt: time.Now(), Error: fmt.Errorf("OFF write failed"),
				}}:
				case <-actorContext.Done():
					return
				}
			}
		}
	}()
	done := startController(ctx, reg, actor, mqttActor{}, modbusActor{}, controller.RuntimeOptions{FeedbackContext: actorContext})
	cancelController()
	err := waitForShutdown(ctx, cancelActors, done, actor, mqttActor{}, modbusActor{}, shutdownOptions{
		cancelController: cancelController, period: time.Second,
	})
	require.ErrorContains(t, err, `cover "unit.cover.office" shutdown: outputs unconfirmed OFF at deadline`)
	select {
	case <-actor.done:
	default:
		require.FailNow(t, "actor did not terminate normally")
	}
}
