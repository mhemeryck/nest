package nest

import (
	"context"
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
) <-chan struct{} {
	done := make(chan struct{})
	go controller.Run(
		ctx,
		reg,
		sysfsActor.commands,
		mqttActor.commands,
		modbusActor.commands,
		mqttActor.topics,
		sysfsActor.states,
		mqttActor.events,
		modbusActor.events,
		done,
		options,
	)

	return done
}

func waitForShutdown(
	ctx context.Context,
	cancel context.CancelFunc,
	controllerDone <-chan struct{},
	sysfsActor sysfsActor,
	mqttActor mqttActor,
	modbusActor modbusActor,
	options ...shutdownOptions,
) error {
	var settings shutdownOptions
	if len(options) > 0 {
		settings = options[0]
	}
	requestShutdown := cancel
	if settings.cancelController != nil {
		requestShutdown = settings.cancelController
	}
	shutdownSignal := ctx.Done()
	sysfsDone, mqttDone, modbusDone := sysfsActor.done, mqttActor.done, modbusActor.done
	var started time.Time
	actorsCancelled := false
	shutdownRequested := false
	var shutdownErr error
	waiting := true
	for waiting {
		select {
		case <-shutdownSignal:
			shutdownSignal = nil
			if started.IsZero() {
				started = time.Now()
			}
		case <-controllerDone:
			waiting = false
		case <-sysfsDone:
			sysfsDone = nil
			if ctx.Err() == nil && !shutdownRequested {
				shutdownErr = fmt.Errorf("sysfs actor stopped")
			}
			if started.IsZero() {
				started = time.Now()
			}
			if !shutdownRequested {
				shutdownRequested = true
				requestShutdown()
			}
			actorsCancelled = settings.cancelController == nil
		case <-mqttDone:
			mqttDone = nil
			if ctx.Err() == nil && !shutdownRequested {
				slog.Error("mqtt actor stopped; local control continues")
			}
		case <-modbusDone:
			modbusDone = nil
			if ctx.Err() == nil && !shutdownRequested {
				shutdownErr = fmt.Errorf("modbus actor stopped")
			}
			if started.IsZero() {
				started = time.Now()
			}
			if !shutdownRequested {
				shutdownRequested = true
				requestShutdown()
			}
			actorsCancelled = settings.cancelController == nil
		}
	}
	if !actorsCancelled {
		cancel()
	}
	if settings.period > 0 {
		if started.IsZero() {
			started = time.Now()
		}
		waitContext, cancelWait := context.WithDeadline(context.WithoutCancel(ctx), started.Add(settings.period))
		defer cancelWait()
		if !waitForActorsWithin(waitContext, sysfsActor, mqttActor, modbusActor, settings.persistence) && shutdownErr == nil {
			shutdownErr = fmt.Errorf("actor shutdown incomplete at deadline")
		}
		return shutdownErr
	}

	waitForSysfsActor(sysfsActor)
	waitForMQTTActor(mqttActor)
	waitForModbusActor(modbusActor)

	return shutdownErr
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
