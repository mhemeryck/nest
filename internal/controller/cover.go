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
	controller := &coverController{
		reg:           reg,
		settings:      registry.CoverControl(reg),
		covers:        make(map[entity.CoverID]*coverRuntime),
		commandOwners: make(map[entity.OutputCommandID]entity.CoverID),
	}
	for _, cover := range registry.Covers(reg) {
		controller.order = append(controller.order, cover.ID)
		controller.covers[cover.ID] = &coverRuntime{
			cover:    cover,
			state:    entity.CoverStateUnknown,
			position: copyPosition(restored[cover.ID]),
			phase:    coverInitializing,
		}
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
		return observeAllCovers(controller, now)
	default:
		return nil
	}
}

func observeAllCovers(controller *coverController, now time.Time) []event.Event {
	var observations []event.Event
	for _, id := range controller.order {
		observations = append(observations, coverObservation(controller.covers[id], now, controller.settings.FullTravelDuration, event.CoverObservationKind))
	}
	return observations
}

func handleCoverRequest(controller *coverController, request event.Cover, now time.Time) []event.Event {
	runtime, ok := controller.covers[request.CoverID]
	if !ok {
		return nil
	}
	switch request.Action {
	case entity.CoverActionStop:
		return handleCoverStopRequest(controller, runtime, now)
	case entity.CoverActionOpen, entity.CoverActionClose:
		return handleCoverDirectionRequest(controller, runtime, request.Action, now)
	default:
		return nil
	}
}

func handleCoverStopRequest(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	switch runtime.phase {
	case coverPreparing, coverActivating, coverMoving:
		return stopCover(controller, runtime, now)
	case coverInitializing, coverIdle, coverStopping, coverRecovering:
		return nil
	default:
		return nil
	}
}

func handleCoverDirectionRequest(controller *coverController, runtime *coverRuntime, direction entity.CoverAction, now time.Time) []event.Event {
	if controller.shuttingDown || runtime.fault != nil {
		return nil
	}
	switch runtime.phase {
	case coverIdle:
		return prepareCoverMovement(controller, runtime, direction, now)
	case coverPreparing, coverActivating, coverMoving:
		switch direction {
		case runtime.direction:
			return nil
		default:
			return stopCover(controller, runtime, now)
		}
	case coverInitializing, coverStopping, coverRecovering:
		return nil
	default:
		return nil
	}
}

func prepareCoverMovement(controller *coverController, runtime *coverRuntime, direction entity.CoverAction, now time.Time) []event.Event {
	runtime.direction = direction
	runtime.initialPosition = copyPosition(runtime.position)
	runtime.activatedAt = time.Time{}
	runtime.activeOffAt = time.Time{}
	runtime.activationID = 0
	runtime.stopReason = ""
	events := []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverStartIntentKind)}
	return append(events, commandBothCoverOutputsOff(controller, runtime, coverPreparing, now)...)
}

func stopCover(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	switch runtime.phase {
	case coverMoving:
		runtime.stopReason = coverEarlyStop
	default:
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
	switch runtime.phase {
	case coverStopping:
		if events, handled := handleCancelledCoverActivation(controller, runtime, result, now); handled {
			return events
		}
	}
	operation, ok := matchCoverOutputResult(controller, runtime, result)
	if !ok {
		return nil
	}
	completePendingCoverOutput(controller, runtime, result.CommandID)
	switch result.Error {
	case nil:
		switch operation.action {
		case entity.OutputActionOn:
			return completeCoverActivation(controller, runtime, result.CompletedAt, now)
		default:
			return handleCoverOffCompletion(controller, runtime, operation.relay, result.CompletedAt, now)
		}
	default:
		return handleCoverOutputFailure(controller, runtime, operation, result.Error, now)
	}
}

func matchCoverOutputResult(controller *coverController, runtime *coverRuntime, result event.OutputResult) (pendingCoverOutput, bool) {
	operation, ok := runtime.pending[result.CommandID]
	if !ok || operation.generation != runtime.generation {
		return pendingCoverOutput{}, false
	}
	relay, ok := registry.RelayByID(controller.reg, operation.relay)
	return operation, ok && relay.SysfsDevice == result.SysfsDevice && operation.action == result.Action
}

func completePendingCoverOutput(controller *coverController, runtime *coverRuntime, id entity.OutputCommandID) {
	delete(runtime.pending, id)
	// Activation correlation remains available until confirmed switch-off
	if id != runtime.activationID {
		delete(controller.commandOwners, id)
	}
}

func handleCancelledCoverActivation(controller *coverController, runtime *coverRuntime, result event.OutputResult, now time.Time) ([]event.Event, bool) {
	// Caller restricts cancelled activation evidence to the stopping phase
	if result.CommandID != runtime.activationID || !runtime.activatedAt.IsZero() || runtime.fault != nil || result.Action != entity.OutputActionOn {
		return nil, false
	}
	relay, ok := registry.RelayByID(controller.reg, activeCoverRelay(runtime))
	if !ok || relay.SysfsDevice != result.SysfsDevice {
		return nil, false
	}
	// Timing and failure evidence; cancelled activation never restores movement intent
	switch result.Error {
	case nil:
		runtime.activatedAt = result.CompletedAt
		return nil, true
	default:
		runtime.fault = result.Error
		runtime.phase = coverRecovering
		runtime.position = nil
		runtime.stopReason = coverFailure
		return []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind)}, true
	}
}

func handleCoverOutputFailure(controller *coverController, runtime *coverRuntime, operation pendingCoverOutput, failure error, now time.Time) []event.Event {
	switch operation.action {
	case entity.OutputActionOn:
		return faultCover(controller, runtime, failure, now, true)
	default:
		switch runtime.phase {
		case coverPreparing:
			return faultCover(controller, runtime, failure, now, true)
		default:
			return recoverFailedCoverSwitchOff(controller, runtime, failure, now)
		}
	}
}

func recoverFailedCoverSwitchOff(controller *coverController, runtime *coverRuntime, failure error, now time.Time) []event.Event {
	recordCoverOutputFault(runtime, failure)
	runtime.attemptFailed = true
	if len(runtime.pending) == 0 {
		runtime.retryAt = now.Add(controller.settings.OffRetryInterval)
	}
	return []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind)}
}

func completeCoverActivation(controller *coverController, runtime *coverRuntime, completedAt, now time.Time) []event.Event {
	runtime.activatedAt = completedAt
	runtime.phase = coverMoving
	switch runtime.direction {
	case entity.CoverActionClose:
		runtime.state = entity.CoverStateClosing
	default:
		runtime.state = entity.CoverStateOpening
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
	switch runtime.phase {
	case coverPreparing:
		return activatePreparedCover(controller, runtime, now)
	case coverInitializing, coverStopping, coverRecovering:
		return completeCoverStop(controller, runtime, now)
	default:
		return completeCoverStop(controller, runtime, now)
	}
}

func activatePreparedCover(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	runtime.phase = coverActivating
	runtime.generation++
	command := commandCoverOutput(controller, runtime, activeCoverRelay(runtime), entity.OutputActionOn, now)
	runtime.activationID = command.OutputCommand.CommandID
	return []event.Event{command}
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
	switch runtime.direction {
	case entity.CoverActionClose:
		return runtime.cover.CloseRelay
	default:
		return runtime.cover.OpenRelay
	}
}

func faultCover(controller *coverController, runtime *coverRuntime, failure error, now time.Time, immediateOff bool) []event.Event {
	recordCoverOutputFault(runtime, failure)
	runtime.travelDeadline = time.Time{}
	runtime.reportDeadline = time.Time{}
	events := beginCoverOutputRecovery(controller, runtime, now, immediateOff)
	return append(events, coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind))
}

func recordCoverOutputFault(runtime *coverRuntime, failure error) {
	runtime.fault = failure
	switch runtime.stopReason {
	case coverCompleted:
		// Completed travel retains its endpoint through switch-off recovery
	default:
		runtime.position = nil
		runtime.stopReason = coverFailure
	}
	runtime.phase = coverRecovering
}

func beginCoverOutputRecovery(controller *coverController, runtime *coverRuntime, now time.Time, immediateOff bool) []event.Event {
	if immediateOff {
		return commandBothCoverOutputsOff(controller, runtime, coverRecovering, now)
	}
	clearPendingCoverOutputs(controller, runtime)
	runtime.retryAt = now.Add(controller.settings.OffRetryInterval)
	return nil
}

func completeCoverStop(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	resolveStoppedCoverState(runtime, now, controller.settings.FullTravelDuration)
	clearCompletedCoverMovement(controller, runtime)
	return []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverStoppedKind)}
}

func resolveStoppedCoverState(runtime *coverRuntime, now time.Time, duration time.Duration) {
	switch runtime.stopReason {
	case coverCompleted:
		resolveCompletedCoverEndpoint(runtime)
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
		runtime.position = estimateCoverPosition(runtime, cutoff, duration)
		runtime.state = entity.CoverStateStopped
	}
}

func resolveCompletedCoverEndpoint(runtime *coverRuntime) {
	var position float64
	switch runtime.direction {
	case entity.CoverActionClose:
		position = 0
		runtime.state = entity.CoverStateClosed
	default:
		position = 100
		runtime.state = entity.CoverStateOpen
	}
	runtime.position = &position
}

func clearCompletedCoverMovement(controller *coverController, runtime *coverRuntime) {
	runtime.phase = coverIdle
	runtime.fault = nil
	runtime.pending = nil
	runtime.retryAt = time.Time{}
	runtime.travelDeadline = time.Time{}
	runtime.reportDeadline = time.Time{}
	runtime.activatedAt = time.Time{}
	delete(controller.commandOwners, runtime.activationID)
	runtime.activationID = 0
}

func handleCoverInputFailure(controller *coverController, inputID entity.DigitalInputID, now time.Time) []event.Event {
	var events []event.Event
	seen := make(map[entity.CoverID]bool)
	for _, button := range registry.PushButtonsByInput(controller.reg, inputID) {
		events = append(events, stopLocalCoversForFailedButton(controller, button.ID, seen, now)...)
		events = append(events, stopRemoteCoversForFailedButton(controller, button.ID, seen)...)
	}
	return events
}

func stopLocalCoversForFailedButton(controller *coverController, buttonID entity.PushButtonID, seen map[entity.CoverID]bool, now time.Time) []event.Event {
	var events []event.Event
	for _, binding := range registry.BindingsByButton(controller.reg, buttonID) {
		id := entity.CoverID(binding.Target)
		runtime, ok := controller.covers[id]
		if !ok || seen[id] {
			continue
		}
		switch runtime.phase {
		case coverPreparing, coverActivating, coverMoving:
			seen[id] = true
			events = append(events, stopCover(controller, runtime, now)...)
		}
	}
	return events
}

func stopRemoteCoversForFailedButton(controller *coverController, buttonID entity.PushButtonID, seen map[entity.CoverID]bool) []event.Event {
	var events []event.Event
	for _, binding := range registry.RemoteSourceBindingsByButton(controller.reg, buttonID) {
		id := entity.CoverID(binding.Target)
		if !seen[id] && entity.IsID(string(id), entity.TypeCover) && binding.ExecutionTransport == entity.ExecutionTransportMQTT {
			seen[id] = true
			events = append(events, event.Event{Kind: event.CoverKind, Cover: &event.Cover{CoverID: id, Action: entity.CoverActionStop}})
		}
	}
	return events
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
	switch runtime.direction {
	case entity.CoverActionClose:
		delta = -delta
	}
	position := math.Max(0, math.Min(100, *runtime.initialPosition+delta))
	return &position
}

func coverObservation(runtime *coverRuntime, now time.Time, duration time.Duration, kind event.Kind) event.Event {
	position := copyPosition(runtime.position)
	switch runtime.phase {
	case coverMoving, coverStopping:
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
		events = append(events, shutDownCover(controller, controller.covers[id], now)...)
	}
	return events
}

func shutDownCover(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	events := []event.Event{coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverStartIntentKind)}
	resolveCoverShutdownReason(runtime, now)
	runtime.travelDeadline = time.Time{}
	runtime.reportDeadline = time.Time{}
	switch runtime.fault {
	case nil:
		return append(events, commandBothCoverOutputsOff(controller, runtime, coverStopping, now)...)
	default:
		return append(events, commandBothCoverOutputsOff(controller, runtime, coverRecovering, now)...)
	}
}

func resolveCoverShutdownReason(runtime *coverRuntime, now time.Time) {
	switch runtime.phase {
	case coverMoving:
		runtime.stopReason = coverEarlyStop
		if !now.Before(runtime.travelDeadline) {
			runtime.stopReason = coverCompleted
		}
	case coverPreparing, coverActivating:
		runtime.stopReason = coverCancelled
	case coverIdle:
		runtime.stopReason = coverEarlyStop
	case coverInitializing, coverStopping, coverRecovering:
		// Retain the existing startup, stop, or recovery outcome
	}
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
