package nest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mhemeryck/nest/internal/controller"
	"github.com/mhemeryck/nest/internal/registry"
)

type runtimeActors struct {
	sysfs       sysfsActor
	mqtt        mqttActor
	modbus      modbusActor
	persistence persistenceActor
}

func newRuntimeActors(reg *registry.Registry) (runtimeActors, error) {
	mqttActor := newMQTTActor(reg)
	sysfsActor, err := newSysfsActor(reg)
	if err != nil {
		return runtimeActors{}, err
	}
	return runtimeActors{
		mqtt: mqttActor, sysfs: sysfsActor,
		modbus: newModbusActor(reg), persistence: newPersistenceActor(reg),
	}, nil
}

func startRuntimeActors(ctx context.Context, actors runtimeActors) (context.Context, context.CancelFunc) {
	actorContext, cancel := context.WithCancel(context.WithoutCancel(ctx))
	startMQTTActor(actorContext, actors.mqtt)
	startSysfsActor(actorContext, actors.sysfs)
	startModbusActor(actorContext, actors.modbus)
	startPersistenceActor(actorContext, actors.persistence)
	return actorContext, cancel
}

func startRuntimeController(ctx, actorContext context.Context, reg *registry.Registry, actors runtimeActors) <-chan error {
	options := controller.RuntimeOptions{
		Persistence: actors.persistence.store, RestoredPositions: actors.persistence.positions,
		FeedbackContext: actorContext,
	}
	return startController(ctx, reg, actors.sysfs, actors.mqtt, actors.modbus, options)
}

func waitForRuntimeShutdown(ctx context.Context, cancelController, cancelActors context.CancelFunc, controllerDone <-chan error, reg *registry.Registry, actors runtimeActors) error {
	settings := shutdownOptions{
		cancelController: cancelController, period: registry.CoverControl(reg).ShutdownPeriod,
		persistence: actors.persistence,
	}
	return waitForShutdown(ctx, cancelActors, controllerDone, actors.sysfs, actors.mqtt, actors.modbus, settings)
}

func startController(
	ctx context.Context,
	reg *registry.Registry,
	sysfsActor sysfsActor,
	mqttActor mqttActor,
	modbusActor modbusActor,
	options controller.RuntimeOptions,
) <-chan error {
	done := make(chan error, 1)
	go func() {
		defer close(done)
		done <- controller.Run(
			ctx,
			reg,
			sysfsActor.commands,
			mqttActor.commands,
			modbusActor.commands,
			mqttActor.topics,
			sysfsActor.states,
			mqttActor.events,
			modbusActor.events,
			options,
		)
	}()

	return done
}

func waitForShutdown(
	ctx context.Context,
	cancel context.CancelFunc,
	controllerDone <-chan error,
	sysfsActor sysfsActor,
	mqttActor mqttActor,
	modbusActor modbusActor,
	settings shutdownOptions,
) error {
	state := newShutdownState(cancel, settings)
	observeRuntimeTermination(ctx, state, controllerDone, sysfsActor, mqttActor, modbusActor, settings)
	if !state.actorsCancelled {
		cancel()
	}
	return finishActorShutdown(ctx, state, sysfsActor, mqttActor, modbusActor, settings)
}

type shutdownState struct {
	requestController context.CancelFunc
	started           time.Time
	actorsCancelled   bool
	requested         bool
	err               error
}

func newShutdownState(cancelActors context.CancelFunc, settings shutdownOptions) *shutdownState {
	requestShutdown := cancelActors
	if settings.cancelController != nil {
		requestShutdown = settings.cancelController
	}
	return &shutdownState{requestController: requestShutdown}
}

func observeRuntimeTermination(ctx context.Context, state *shutdownState, controllerDone <-chan error, sysfsActor sysfsActor, mqttActor mqttActor, modbusActor modbusActor, settings shutdownOptions) {
	shutdownSignal := ctx.Done()
	sysfsDone, mqttDone, modbusDone := sysfsActor.done, mqttActor.done, modbusActor.done
	for {
		select {
		case <-shutdownSignal:
			shutdownSignal = nil
			recordShutdownStart(state)
		case err := <-controllerDone:
			state.err = errors.Join(state.err, err)
			return
		case <-sysfsDone:
			sysfsDone = nil
			handleRequiredActorTermination(ctx, state, "sysfs", settings)
		case <-mqttDone:
			mqttDone = nil
			handleMQTTActorTermination(ctx, state)
		case <-modbusDone:
			modbusDone = nil
			handleRequiredActorTermination(ctx, state, "modbus", settings)
		}
	}
}

func handleRequiredActorTermination(ctx context.Context, state *shutdownState, actor string, settings shutdownOptions) {
	recordRequiredActorFailure(ctx, state, actor)
	requestControllerShutdown(state, settings)
}

func recordRequiredActorFailure(ctx context.Context, state *shutdownState, actor string) {
	if ctx.Err() == nil && !state.requested {
		state.err = fmt.Errorf("%s actor stopped", actor)
	}
}

func requestControllerShutdown(state *shutdownState, settings shutdownOptions) {
	recordShutdownStart(state)
	if !state.requested {
		state.requested = true
		state.requestController()
	}
	state.actorsCancelled = settings.cancelController == nil
}

func recordShutdownStart(state *shutdownState) {
	if state.started.IsZero() {
		state.started = time.Now()
	}
}

func handleMQTTActorTermination(ctx context.Context, state *shutdownState) {
	if ctx.Err() == nil && !state.requested {
		slog.Error("mqtt actor stopped; local control continues")
	}
}

func finishActorShutdown(ctx context.Context, state *shutdownState, sysfsActor sysfsActor, mqttActor mqttActor, modbusActor modbusActor, settings shutdownOptions) error {
	if settings.period > 0 {
		return finishBoundedActorShutdown(ctx, state, sysfsActor, mqttActor, modbusActor, settings)
	}

	waitForSysfsActor(sysfsActor)
	waitForMQTTActor(mqttActor)
	waitForModbusActor(modbusActor)

	return state.err
}

func finishBoundedActorShutdown(ctx context.Context, state *shutdownState, sysfsActor sysfsActor, mqttActor mqttActor, modbusActor modbusActor, settings shutdownOptions) error {
	recordShutdownStart(state)
	waitContext, cancelWait := context.WithDeadline(context.WithoutCancel(ctx), state.started.Add(settings.period))
	defer cancelWait()
	if !waitForActorsWithin(waitContext, sysfsActor, mqttActor, modbusActor, settings.persistence) {
		state.err = errors.Join(state.err, fmt.Errorf("actor shutdown incomplete at deadline"))
	}
	return state.err
}

type shutdownOptions struct {
	cancelController context.CancelFunc
	period           time.Duration
	persistence      persistenceActor
}

func waitForActorsWithin(ctx context.Context, sysfsActor sysfsActor, mqttActor mqttActor, modbusActor modbusActor, persistenceActor persistenceActor) bool {
	sysfsDone, mqttDone, modbusDone, persistenceDone := sysfsActor.done, mqttActor.done, modbusActor.done, persistenceActor.done
	for sysfsDone != nil || mqttDone != nil || modbusDone != nil || persistenceDone != nil {
		select {
		case <-sysfsDone:
			close(sysfsActor.states)
			sysfsDone = nil
		case <-mqttDone:
			mqttDone = nil
		case <-modbusDone:
			close(modbusActor.events)
			modbusDone = nil
		case <-persistenceDone:
			persistenceDone = nil
		case <-ctx.Done():
			return false
		}
	}
	return true
}
