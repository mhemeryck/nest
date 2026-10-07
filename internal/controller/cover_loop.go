package controller

import (
	"context"
	"log/slog"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/mhemeryck/nest/internal/mqtt"
	"github.com/mhemeryck/nest/internal/persistence"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/mhemeryck/nest/internal/sysfs"
)

type coverLoop struct {
	covers           *coverController
	queue            []event.Event
	shutdownSignal   <-chan struct{}
	shutdownDeadline time.Time
}

// runCoverLoop processes semantic events and cover deadlines sequentially.
// It keeps output feedback available during staged shutdown and returns an error
// if cover outputs remain unconfirmed OFF at the shutdown deadline.
func runCoverLoop(
	ctx context.Context,
	reg *registry.Registry,
	sysfsCommands chan<- sysfs.Command,
	mqttCommands chan<- mqtt.Command,
	modbusCommands chan<- modbus.Command,
	mqttTopics mqtt.Topics,
	semanticEvents <-chan event.Event,
	runtimeOptions RuntimeOptions,
) error {
	feedbackContext := runtimeOptions.FeedbackContext
	if feedbackContext == nil {
		feedbackContext = context.WithoutCancel(ctx)
	}
	handoffs, stopHandoffs := startDispatchHandoffs(feedbackContext, reg, mqttCommands, modbusCommands)
	defer stopHandoffs()
	targets := dispatchTargets{
		reg: reg, sysfsCommands: sysfsCommands, mqttCommands: mqttCommands,
		modbusCommands: modbusCommands, mqttTopics: mqttTopics,
		persistence: runtimeOptions.Persistence, handoffs: handoffs,
	}
	loop := newCoverLoop(ctx, reg, runtimeOptions.RestoredPositions, time.Now())
	timer := time.NewTimer(time.Hour)
	// Inactive until armed for the next deadline; initial duration unused
	timer.Stop()
	defer timer.Stop()
	semanticEventsOpen := true

	for {
		now := time.Now()
		select {
		case <-loop.shutdownSignal:
			beginCoverLoopShutdown(loop, now)
		default:
		}
		if loop.covers.shuttingDown && !now.Before(loop.shutdownDeadline) {
			return coverShutdownError(loop.covers)
		}
		due := processCoverDeadlines(loop.covers, time.Now())
		loop.queue = append(due, loop.queue...)
		if dispatchNextCoverLoopEvent(feedbackContext, loop, targets) {
			continue
		}

		if loop.covers.shuttingDown && coverOutputsStopped(loop.covers) {
			finishCoverLoopShutdown(feedbackContext, runtimeOptions.Persistence, loop.shutdownDeadline)
			return nil
		}
		if !semanticEventsOpen && !loop.covers.shuttingDown {
			beginCoverLoopShutdown(loop, time.Now())
			continue
		}

		var deadline <-chan time.Time
		next := nextCoverLoopDeadline(loop)
		if !next.IsZero() {
			timer.Reset(max(0, time.Until(next)))
			deadline = timer.C
		}
		select {
		case <-loop.shutdownSignal:
			beginCoverLoopShutdown(loop, time.Now())
		case <-deadline:
		case semanticEvent, ok := <-semanticEvents:
			if !ok {
				semanticEventsOpen = false
				semanticEvents = nil
				continue
			}
			loop.queue = append(loop.queue, semanticEvent)
		}
	}
}

func newCoverLoop(ctx context.Context, reg *registry.Registry, restored map[entity.CoverID]*float64, now time.Time) *coverLoop {
	covers := newCoverController(reg, restored)
	return &coverLoop{covers: covers, queue: initializeCovers(covers, now), shutdownSignal: ctx.Done()}
}

func dispatchNextCoverLoopEvent(ctx context.Context, loop *coverLoop, targets dispatchTargets) bool {
	incoming, remaining, ok := popEvent(loop.queue)
	if !ok {
		return false
	}
	loop.queue = remaining
	if incoming.Kind == event.OutputCommandKind && !currentCoverCommand(loop.covers, *incoming.OutputCommand) {
		return true
	}
	derived := handleCoverEvent(loop.covers, incoming, time.Now())
	derived = append(derived, dispatchPersistenceEvent(targets.persistence, incoming)...)
	derived = append(derived, dispatchEvent(ctx, targets.reg, targets.sysfsCommands, targets.mqttCommands,
		targets.modbusCommands, targets.mqttTopics, incoming, targets.handoffs)...)
	loop.queue = append(loop.queue, derived...)
	return true
}

func beginCoverLoopShutdown(loop *coverLoop, now time.Time) {
	loop.shutdownSignal = nil
	loop.shutdownDeadline = now.Add(loop.covers.settings.ShutdownPeriod)
	loop.queue = append(beginCoverShutdown(loop.covers, now), loop.queue...)
}

func finishCoverLoopShutdown(ctx context.Context, store *persistence.Store, deadline time.Time) {
	dispatchPersistenceEvent(store, event.Event{Kind: event.SessionStoppedKind})
	if store == nil {
		return
	}
	flushContext, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	if !persistence.FlushWithin(flushContext, store) {
		slog.Error("persistence shutdown flush incomplete")
	}
}

func nextCoverLoopDeadline(loop *coverLoop) time.Time {
	next := nextCoverDeadline(loop.covers)
	if loop.covers.shuttingDown && (next.IsZero() || loop.shutdownDeadline.Before(next)) {
		return loop.shutdownDeadline
	}
	return next
}
