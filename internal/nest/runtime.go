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
	requestShutdown := cancel
	if settings.cancelController != nil {
		requestShutdown = settings.cancelController
	}
	state := &shutdownState{requestController: requestShutdown}
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

func observeRuntimeTermination(ctx context.Context, state *shutdownState, controllerDone <-chan error, sysfsActor sysfsActor, mqttActor mqttActor, modbusActor modbusActor, settings shutdownOptions) {
	shutdownSignal := ctx.Done()
	sysfsDone, mqttDone, modbusDone := sysfsActor.done, mqttActor.done, modbusActor.done
	for {
		select {
		case <-shutdownSignal:
			shutdownSignal = nil
			if state.started.IsZero() {
				state.started = time.Now()
			}
		case err := <-controllerDone:
			state.err = errors.Join(state.err, err)
			return
		case <-sysfsDone:
			sysfsDone = nil
			handleRequiredActorTermination(ctx, state, "sysfs", settings)
		case <-mqttDone:
			mqttDone = nil
			if ctx.Err() == nil && !state.requested {
				slog.Error("mqtt actor stopped; local control continues")
			}
		case <-modbusDone:
			modbusDone = nil
			handleRequiredActorTermination(ctx, state, "modbus", settings)
		}
	}
}

func handleRequiredActorTermination(ctx context.Context, state *shutdownState, actor string, settings shutdownOptions) {
	if ctx.Err() == nil && !state.requested {
		state.err = fmt.Errorf("%s actor stopped", actor)
	}
	if state.started.IsZero() {
		state.started = time.Now()
	}
	if !state.requested {
		state.requested = true
		state.requestController()
	}
	state.actorsCancelled = settings.cancelController == nil
}

func finishActorShutdown(ctx context.Context, state *shutdownState, sysfsActor sysfsActor, mqttActor mqttActor, modbusActor modbusActor, settings shutdownOptions) error {
	if settings.period > 0 {
		if state.started.IsZero() {
			state.started = time.Now()
		}
		waitContext, cancelWait := context.WithDeadline(context.WithoutCancel(ctx), state.started.Add(settings.period))
		defer cancelWait()
		if !waitForActorsWithin(waitContext, sysfsActor, mqttActor, modbusActor, settings.persistence) {
			state.err = errors.Join(state.err, fmt.Errorf("actor shutdown incomplete at deadline"))
		}
		return state.err
	}

	waitForSysfsActor(sysfsActor)
	waitForMQTTActor(mqttActor)
	waitForModbusActor(modbusActor)

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
