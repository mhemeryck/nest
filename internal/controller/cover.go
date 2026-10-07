package controller

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
)

type coverPhase string

const (
	coverInitializing coverPhase = "initializing"
	coverIdle         coverPhase = "idle"
	coverPreparing    coverPhase = "preparing"
	coverActivating   coverPhase = "activating"
	coverMoving       coverPhase = "moving"
	coverStopping     coverPhase = "stopping"
	coverRecovering   coverPhase = "recovering"
)

type coverStopReason string

const (
	coverStartup   coverStopReason = "startup"
	coverEarlyStop coverStopReason = "early_stop"
	coverCompleted coverStopReason = "completed"
	coverCancelled coverStopReason = "cancelled"
	coverFailure   coverStopReason = "failure"
)

type pendingCoverOutput struct {
	relay      entity.RelayID
	action     entity.OutputAction
	generation uint64
	deadline   time.Time
}

type coverRuntime struct {
	cover           entity.Cover
	state           entity.CoverState
	position        *float64
	initialPosition *float64
	phase           coverPhase
	direction       entity.CoverAction
	fault           error
	generation      uint64
	pending         map[entity.OutputCommandID]pendingCoverOutput
	activationID    entity.OutputCommandID
	activatedAt     time.Time
	activeOffAt     time.Time
	travelDeadline  time.Time
	reportDeadline  time.Time
	retryAt         time.Time
	stopReason      coverStopReason
	attemptFailed   bool
}

type coverController struct {
	reg           *registry.Registry
	settings      entity.CoverControl
	covers        map[entity.CoverID]*coverRuntime
	order         []entity.CoverID
	commandOwners map[entity.OutputCommandID]entity.CoverID
	nextCommandID entity.OutputCommandID
	shuttingDown  bool
}

func newCoverController(reg *registry.Registry, restored map[entity.CoverID]*float64) *coverController {
	controller := &coverController{reg: reg, settings: registry.CoverControl(reg), covers: make(map[entity.CoverID]*coverRuntime), commandOwners: make(map[entity.OutputCommandID]entity.CoverID)}
	for _, cover := range registry.Covers(reg) {
		controller.order = append(controller.order, cover.ID)
		controller.covers[cover.ID] = &coverRuntime{cover: cover, state: entity.CoverStateUnknown, position: copyPosition(restored[cover.ID]), phase: coverInitializing}
	}
	return controller
}

func initializeCovers(controller *coverController, now time.Time) []event.Event {
	events := []event.Event{{Kind: event.SessionStartedKind}}
	for _, id := range controller.order {
		runtime := controller.covers[id]
		runtime.stopReason = coverStartup
		events = append(events, commandBothCoverOutputsOff(controller, runtime, coverInitializing, now)...)
		events = append(events, coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind))
	}
	return events
}

func handleCoverEvent(controller *coverController, incoming event.Event, now time.Time) []event.Event {
	switch incoming.Kind {
	case event.CoverKind:
		return handleCoverRequest(controller, *incoming.Cover, now)
	case event.OutputResultKind:
		return handleCoverOutputResult(controller, *incoming.OutputResult, now)
	case event.InputFailureKind:
		return handleCoverInputFailure(controller, incoming.InputFailure.InputID, now)
	case event.MQTTConnectedKind:
		var observations []event.Event
		for _, id := range controller.order {
			observations = append(observations, coverObservation(controller.covers[id], now, controller.settings.FullTravelDuration, event.CoverObservationKind))
		}
		return observations
	default:
		return nil
	}
}

func handleCoverRequest(controller *coverController, request event.Cover, now time.Time) []event.Event {
	runtime, ok := controller.covers[request.CoverID]
	if !ok {
		return nil
	}
	if request.Action != entity.CoverActionOpen && request.Action != entity.CoverActionClose && request.Action != entity.CoverActionStop {
		return nil
	}
	if request.Action == entity.CoverActionStop {
		if runtime.phase == coverMoving || runtime.phase == coverPreparing || runtime.phase == coverActivating {
			return stopCover(controller, runtime, now)
		}
		return nil
	}
	if controller.shuttingDown || runtime.fault != nil {
		return nil
	}
	switch runtime.phase {
	case coverIdle:
		runtime.direction = request.Action
		runtime.initialPosition = copyPosition(runtime.position)
		runtime.activatedAt = time.Time{}
		runtime.activeOffAt = time.Time{}
		runtime.activationID = 0
		runtime.stopReason = ""
		events := []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverStartIntentKind)}
		return append(events, commandBothCoverOutputsOff(controller, runtime, coverPreparing, now)...)
	case coverPreparing, coverActivating, coverMoving:
		if request.Action != runtime.direction {
			return stopCover(controller, runtime, now)
		}
	}
	return nil
}

func stopCover(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	if runtime.phase == coverMoving {
		runtime.stopReason = coverEarlyStop
	} else {
		runtime.stopReason = coverCancelled
	}
	runtime.travelDeadline = time.Time{}
	runtime.reportDeadline = time.Time{}
	return commandBothCoverOutputsOff(controller, runtime, coverStopping, now)
}

func commandBothCoverOutputsOff(controller *coverController, runtime *coverRuntime, phase coverPhase, now time.Time) []event.Event {
	runtime.phase = phase
	runtime.generation++
	clearPendingCoverOutputs(controller, runtime)
	runtime.pending = make(map[entity.OutputCommandID]pendingCoverOutput)
	runtime.attemptFailed = false
	runtime.retryAt = time.Time{}
	return []event.Event{
		commandCoverOutput(controller, runtime, runtime.cover.OpenRelay, entity.OutputActionOff, now),
		commandCoverOutput(controller, runtime, runtime.cover.CloseRelay, entity.OutputActionOff, now),
	}
}

func commandCoverOutput(controller *coverController, runtime *coverRuntime, relay entity.RelayID, action entity.OutputAction, now time.Time) event.Event {
	controller.nextCommandID++
	id := controller.nextCommandID
	controller.commandOwners[id] = runtime.cover.ID
	runtime.pending[id] = pendingCoverOutput{relay: relay, action: action, generation: runtime.generation, deadline: now.Add(controller.settings.OperationTimeout)}
	return event.Event{Kind: event.OutputCommandKind, OutputCommand: &event.OutputCommand{CommandID: id, RelayID: relay, Action: action}}
}

func handleCoverOutputResult(controller *coverController, result event.OutputResult, now time.Time) []event.Event {
	id, owned := controller.commandOwners[result.CommandID]
	if !owned {
		return nil
	}
	runtime := controller.covers[id]
	if events, handled := handleCancelledCoverActivation(controller, runtime, result, now); handled {
		return events
	}
	operation, ok := runtime.pending[result.CommandID]
	if !ok || operation.generation != runtime.generation {
		return nil
	}
	relay, ok := registry.RelayByID(controller.reg, operation.relay)
	if !ok || relay.SysfsDevice != result.SysfsDevice || operation.action != result.Action {
		return nil
	}
	delete(runtime.pending, result.CommandID)
	if result.CommandID != runtime.activationID {
		delete(controller.commandOwners, result.CommandID)
	}
	if result.Error != nil {
		return handleCoverOutputFailure(controller, runtime, operation, result.Error, now)
	}
	if operation.action == entity.OutputActionOn {
		return completeCoverActivation(controller, runtime, result.CompletedAt, now)
	}
	return handleCoverOffCompletion(controller, runtime, operation.relay, result.CompletedAt, now)
}

func handleCancelledCoverActivation(controller *coverController, runtime *coverRuntime, result event.OutputResult, now time.Time) ([]event.Event, bool) {
	if result.CommandID != runtime.activationID || !runtime.activatedAt.IsZero() || runtime.phase != coverStopping || runtime.fault != nil || result.Action != entity.OutputActionOn {
		return nil, false
	}
	relay, ok := registry.RelayByID(controller.reg, activeCoverRelay(runtime))
	if !ok || relay.SysfsDevice != result.SysfsDevice {
		return nil, false
	}
	// Timing and failure evidence; cancelled activation never restores movement intent
	if result.Error != nil {
		runtime.fault = result.Error
		runtime.phase = coverRecovering
		runtime.position = nil
		runtime.stopReason = coverFailure
		return []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind)}, true
	}
	runtime.activatedAt = result.CompletedAt
	return nil, true
}

func handleCoverOutputFailure(controller *coverController, runtime *coverRuntime, operation pendingCoverOutput, failure error, now time.Time) []event.Event {
	if operation.action == entity.OutputActionOn || runtime.phase == coverPreparing {
		return faultCover(controller, runtime, failure, now, true)
	}
	runtime.fault = failure
	runtime.attemptFailed = true
	runtime.phase = coverRecovering
	if runtime.stopReason != coverCompleted {
		runtime.position = nil
		runtime.stopReason = coverFailure
	}
	if len(runtime.pending) == 0 {
		runtime.retryAt = now.Add(controller.settings.OffRetryInterval)
	}
	return []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind)}
}

func completeCoverActivation(controller *coverController, runtime *coverRuntime, completedAt, now time.Time) []event.Event {
	runtime.activatedAt = completedAt
	runtime.phase = coverMoving
	runtime.state = entity.CoverStateOpening
	if runtime.direction == entity.CoverActionClose {
		runtime.state = entity.CoverStateClosing
	}
	runtime.travelDeadline = completedAt.Add(controller.settings.FullTravelDuration)
	runtime.reportDeadline = now.Add(controller.settings.ReportingInterval)
	return []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind)}
}

func handleCoverOffCompletion(controller *coverController, runtime *coverRuntime, relay entity.RelayID, completedAt, now time.Time) []event.Event {
	if relay == activeCoverRelay(runtime) {
		runtime.activeOffAt = completedAt
	}
	if len(runtime.pending) > 0 {
		return nil
	}
	if runtime.attemptFailed {
		runtime.retryAt = now.Add(controller.settings.OffRetryInterval)
		return nil
	}
	if runtime.phase == coverPreparing {
		runtime.phase = coverActivating
		runtime.generation++
		command := commandCoverOutput(controller, runtime, activeCoverRelay(runtime), entity.OutputActionOn, now)
		runtime.activationID = command.OutputCommand.CommandID
		return []event.Event{command}
	}
	return completeCoverStop(controller, runtime, now)
}

func clearPendingCoverOutputs(controller *coverController, runtime *coverRuntime) {
	for id := range runtime.pending {
		// Retain cancelled activation correlation until switch-off completes
		if id != runtime.activationID {
			delete(controller.commandOwners, id)
		}
	}
	runtime.pending = nil
}

func activeCoverRelay(runtime *coverRuntime) entity.RelayID {
	if runtime.direction == entity.CoverActionClose {
		return runtime.cover.CloseRelay
	}
	return runtime.cover.OpenRelay
}

func faultCover(controller *coverController, runtime *coverRuntime, failure error, now time.Time, immediateOff bool) []event.Event {
	runtime.fault = failure
	runtime.travelDeadline = time.Time{}
	runtime.reportDeadline = time.Time{}
	if runtime.stopReason != coverCompleted {
		runtime.position = nil
		runtime.stopReason = coverFailure
	}
	runtime.phase = coverRecovering
	var events []event.Event
	if immediateOff {
		events = commandBothCoverOutputsOff(controller, runtime, coverRecovering, now)
	} else {
		clearPendingCoverOutputs(controller, runtime)
		runtime.retryAt = now.Add(controller.settings.OffRetryInterval)
	}
	return append(events, coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind))
}

func completeCoverStop(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	switch runtime.stopReason {
	case coverCompleted:
		position := 100.0
		runtime.state = entity.CoverStateOpen
		if runtime.direction == entity.CoverActionClose {
			position = 0
			runtime.state = entity.CoverStateClosed
		}
		runtime.position = &position
	case coverFailure:
		runtime.state = entity.CoverStateStopped
		runtime.position = nil
	case coverStartup:
		runtime.state = entity.CoverStateStopped
	default:
		cutoff := runtime.activeOffAt
		if cutoff.IsZero() {
			cutoff = now
		}
		runtime.position = estimateCoverPosition(runtime, cutoff, controller.settings.FullTravelDuration)
		runtime.state = entity.CoverStateStopped
	}
	runtime.phase = coverIdle
	runtime.fault = nil
	runtime.pending = nil
	runtime.retryAt = time.Time{}
	runtime.travelDeadline = time.Time{}
	runtime.reportDeadline = time.Time{}
	runtime.activatedAt = time.Time{}
	delete(controller.commandOwners, runtime.activationID)
	runtime.activationID = 0
	return []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverStoppedKind)}
}

func handleCoverInputFailure(controller *coverController, inputID entity.DigitalInputID, now time.Time) []event.Event {
	var events []event.Event
	seen := make(map[entity.CoverID]bool)
	for _, button := range registry.PushButtonsByInput(controller.reg, inputID) {
		for _, binding := range registry.BindingsByButton(controller.reg, button.ID) {
			id := entity.CoverID(binding.Target)
			runtime, ok := controller.covers[id]
			if ok && !seen[id] && (runtime.phase == coverMoving || runtime.phase == coverPreparing || runtime.phase == coverActivating) {
				seen[id] = true
				events = append(events, stopCover(controller, runtime, now)...)
			}
		}
		for _, binding := range registry.RemoteSourceBindingsByButton(controller.reg, button.ID) {
			id := entity.CoverID(binding.Target)
			if !seen[id] && entity.IsID(string(id), entity.TypeCover) && binding.ExecutionTransport == entity.ExecutionTransportMQTT {
				seen[id] = true
				events = append(events, event.Event{Kind: event.CoverKind, Cover: &event.Cover{CoverID: id, Action: entity.CoverActionStop}})
			}
		}
	}
	return events
}

func processCoverDeadlines(controller *coverController, now time.Time) []event.Event {
	var events []event.Event
	for _, id := range controller.order {
		runtime := controller.covers[id]
		expired := false
		for _, operation := range runtime.pending {
			if operation.generation == runtime.generation && !now.Before(operation.deadline) {
				expired = true
				break
			}
		}
		if expired {
			immediateOff := runtime.phase == coverActivating || runtime.phase == coverPreparing
			events = append(events, faultCover(controller, runtime, fmt.Errorf("cover output operation timed out"), now, immediateOff)...)
			continue
		}
		if runtime.phase == coverMoving && !now.Before(runtime.travelDeadline) {
			runtime.stopReason = coverCompleted
			runtime.travelDeadline = time.Time{}
			runtime.reportDeadline = time.Time{}
			events = append(events, commandBothCoverOutputsOff(controller, runtime, coverStopping, now)...)
			continue
		}
		if !runtime.retryAt.IsZero() && !now.Before(runtime.retryAt) {
			events = append(events, commandBothCoverOutputsOff(controller, runtime, coverRecovering, now)...)
		}
		if runtime.phase == coverMoving && !now.Before(runtime.reportDeadline) {
			events = append(events, coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind))
			runtime.reportDeadline = now.Add(controller.settings.ReportingInterval)
		}
	}
	return events
}

func nextCoverDeadline(controller *coverController) time.Time {
	var next time.Time
	include := func(deadline time.Time) {
		if !deadline.IsZero() && (next.IsZero() || deadline.Before(next)) {
			next = deadline
		}
	}
	for _, runtime := range controller.covers {
		include(runtime.travelDeadline)
		include(runtime.reportDeadline)
		include(runtime.retryAt)
		for _, operation := range runtime.pending {
			include(operation.deadline)
		}
	}
	return next
}

func estimateCoverPosition(runtime *coverRuntime, now time.Time, duration time.Duration) *float64 {
	if runtime.activatedAt.IsZero() {
		return copyPosition(runtime.position)
	}
	if runtime.initialPosition == nil {
		return nil
	}
	elapsed := max(0, now.Sub(runtime.activatedAt))
	delta := 100 * float64(elapsed) / float64(duration)
	if runtime.direction == entity.CoverActionClose {
		delta = -delta
	}
	position := math.Max(0, math.Min(100, *runtime.initialPosition+delta))
	return &position
}

func coverObservation(runtime *coverRuntime, now time.Time, duration time.Duration, kind event.Kind) event.Event {
	position := copyPosition(runtime.position)
	if runtime.phase == coverMoving || runtime.phase == coverStopping {
		position = estimateCoverPosition(runtime, now, duration)
	}
	if runtime.fault != nil && runtime.stopReason != coverCompleted {
		position = nil
	}
	return event.Event{Kind: kind, CoverObservation: &event.CoverObservation{
		CoverID: runtime.cover.ID, State: runtime.state, EstimatedPosition: position,
		Available: runtime.fault == nil && runtime.phase != coverInitializing, Direction: runtime.direction,
	}}
}

func copyPosition(position *float64) *float64 {
	if position == nil {
		return nil
	}
	value := *position
	return &value
}

func beginCoverShutdown(controller *coverController, now time.Time) []event.Event {
	controller.shuttingDown = true
	var events []event.Event
	for _, id := range controller.order {
		runtime := controller.covers[id]
		events = append(events, coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverStartIntentKind))
		if runtime.phase == coverMoving {
			runtime.stopReason = coverEarlyStop
			if !now.Before(runtime.travelDeadline) {
				runtime.stopReason = coverCompleted
			}
		}
		if runtime.phase == coverPreparing || runtime.phase == coverActivating {
			runtime.stopReason = coverCancelled
		}
		if runtime.phase == coverIdle {
			runtime.stopReason = coverEarlyStop
		}
		phase := coverStopping
		if runtime.fault != nil {
			phase = coverRecovering
		}
		runtime.travelDeadline = time.Time{}
		runtime.reportDeadline = time.Time{}
		events = append(events, commandBothCoverOutputsOff(controller, runtime, phase, now)...)
	}
	return events
}

func coverOutputsStopped(controller *coverController) bool {
	for _, runtime := range controller.covers {
		if runtime.phase != coverIdle || runtime.fault != nil {
			return false
		}
	}
	return true
}

func currentCoverCommand(controller *coverController, command event.OutputCommand) bool {
	id, owned := controller.commandOwners[command.CommandID]
	if !owned {
		return false
	}
	runtime := controller.covers[id]
	operation, exists := runtime.pending[command.CommandID]
	return exists && operation.generation == runtime.generation && operation.relay == command.RelayID && operation.action == command.Action
}

func coverShutdownError(controller *coverController) error {
	var failures error
	for _, id := range controller.order {
		runtime := controller.covers[id]
		if runtime.phase != coverIdle || runtime.fault != nil {
			failures = errors.Join(failures, fmt.Errorf("cover %q shutdown: outputs unconfirmed OFF at deadline", id))
		}
	}
	return failures
}
