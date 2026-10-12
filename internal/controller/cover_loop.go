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
	semanticEvents   <-chan event.Event
	inputClosed      bool
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
	feedbackContext := coverFeedbackContext(ctx, runtimeOptions)
	handoffs, stopHandoffs := startDispatchHandoffs(feedbackContext, reg, mqttCommands, modbusCommands)
	defer stopHandoffs()
	targets := newDispatchTargets(reg, sysfsCommands, mqttCommands, modbusCommands, mqttTopics, runtimeOptions.Persistence, handoffs)
	loop := newCoverLoop(ctx, reg, runtimeOptions.RestoredPositions, time.Now())
	return processCoverLoop(feedbackContext, loop, targets, semanticEvents)
}

func processCoverLoop(ctx context.Context, loop *coverLoop, targets dispatchTargets, semanticEvents <-chan event.Event) error {
	timer := prepareCoverLoopInput(loop, semanticEvents)
	defer timer.Stop()
	runActiveCoverLoop(ctx, loop, targets, timer)
	return drainCoverLoopShutdown(ctx, loop, targets, timer)
}

func runActiveCoverLoop(ctx context.Context, loop *coverLoop, targets dispatchTargets, timer *time.Timer) {
	for !coverLoopShutdownRequested(loop) {
		if processNextCoverLoopWork(ctx, loop, targets) {
			continue
		}
		if loop.inputClosed {
			return
		}
		waitForCoverLoopWork(loop, timer)
	}
}

func drainCoverLoopShutdown(ctx context.Context, loop *coverLoop, targets dispatchTargets, timer *time.Timer) error {
	beginCoverLoopShutdown(loop, time.Now())
	for time.Now().Before(loop.shutdownDeadline) {
		if processNextCoverLoopWork(ctx, loop, targets) {
			continue
		}
		if coverOutputsStopped(loop.covers) {
			finishCoverLoopShutdown(ctx, targets.persistence, loop.shutdownDeadline)
			return nil
		}
		waitForCoverLoopWork(loop, timer)
	}
	return coverShutdownError(loop.covers)
}

func processNextCoverLoopWork(ctx context.Context, loop *coverLoop, targets dispatchTargets) bool {
	queueDueCoverEvents(loop, time.Now())
	return dispatchNextCoverLoopEvent(ctx, loop, targets)
}

func prepareCoverLoopInput(loop *coverLoop, semanticEvents <-chan event.Event) *time.Timer {
	loop.semanticEvents = semanticEvents
	loop.inputClosed = false
	timer := time.NewTimer(time.Hour)
	// Inactive until armed for the next deadline; initial duration unused
	timer.Stop()
	return timer
}

func coverLoopShutdownRequested(loop *coverLoop) bool {
	select {
	case <-loop.shutdownSignal:
		return true
	default:
		return false
	}
}

func queueDueCoverEvents(loop *coverLoop, now time.Time) {
	due := processCoverDeadlines(loop.covers, now)
	loop.queue = append(due, loop.queue...)
}

func armCoverLoopTimer(loop *coverLoop, timer *time.Timer) <-chan time.Time {
	next := nextCoverLoopDeadline(loop)
	if next.IsZero() {
		return nil
	}
	timer.Reset(max(0, time.Until(next)))
	return timer.C
}

func waitForCoverLoopWork(loop *coverLoop, timer *time.Timer) {
	deadline := armCoverLoopTimer(loop, timer)
	select {
	case <-loop.shutdownSignal:
	case <-deadline:
	case semanticEvent, ok := <-loop.semanticEvents:
		admitCoverLoopInput(loop, semanticEvent, ok)
	}
}

func admitCoverLoopInput(loop *coverLoop, incoming event.Event, open bool) {
	if !open {
		loop.inputClosed = true
		loop.semanticEvents = nil
		return
	}
	loop.queue = append(loop.queue, incoming)
}

func newCoverLoop(ctx context.Context, reg *registry.Registry, restored map[entity.CoverID]*float64, now time.Time) *coverLoop {
	covers := newCoverController(reg, restored)
	return &coverLoop{covers: covers, queue: initializeCovers(covers, now), shutdownSignal: ctx.Done()}
}

func dispatchNextCoverLoopEvent(ctx context.Context, loop *coverLoop, targets dispatchTargets) bool {
	incoming, ok := dequeueCoverLoopEvent(loop)
	if !ok {
		return false
	}
	if !currentCoverLoopEvent(loop, incoming) {
		return true
	}
	dispatchCoverLoopEvent(ctx, loop, targets, incoming)
	return true
}

func dequeueCoverLoopEvent(loop *coverLoop) (event.Event, bool) {
	incoming, remaining, ok := popEvent(loop.queue)
	if ok {
		loop.queue = remaining
	}
	return incoming, ok
}

func currentCoverLoopEvent(loop *coverLoop, incoming event.Event) bool {
	switch incoming.Kind {
	case event.OutputCommandKind:
		return currentCoverCommand(loop.covers, *incoming.OutputCommand)
	default:
		return true
	}
}

func dispatchCoverLoopEvent(ctx context.Context, loop *coverLoop, targets dispatchTargets, incoming event.Event) {
	derived := handleCoverEvent(loop.covers, incoming, time.Now())
	derived = append(derived, dispatchPersistenceEvent(targets.persistence, incoming)...)
	derived = append(derived, dispatchEvent(ctx, targets.reg, targets.sysfsCommands, targets.mqttCommands,
		targets.modbusCommands, targets.mqttTopics, incoming, targets.handoffs)...)
	loop.queue = append(loop.queue, derived...)
}

func beginCoverLoopShutdown(loop *coverLoop, now time.Time) {
	loop.shutdownSignal = nil
	loop.shutdownDeadline = now.Add(loop.covers.settings.ShutdownPeriod)
	loop.queue = append(beginCoverShutdown(loop.covers, now), loop.queue...)
}

func finishCoverLoopShutdown(ctx context.Context, store *persistence.Store, deadline time.Time) {
	dispatchPersistenceEvent(store, event.Event{Kind: event.SessionStoppedKind})
	flushCoverPersistenceWithin(ctx, store, deadline)
}

func flushCoverPersistenceWithin(ctx context.Context, store *persistence.Store, deadline time.Time) {
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
