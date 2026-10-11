package controller

import (
	"fmt"
	"time"

	"github.com/mhemeryck/nest/internal/controller/event"
)

func processCoverDeadlines(controller *coverController, now time.Time) []event.Event {
	var events []event.Event
	for _, id := range controller.order {
		events = append(events, processCoverRuntimeDeadlines(controller, controller.covers[id], now)...)
	}
	return events
}

func processCoverRuntimeDeadlines(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	// Output timeout precedes travel completion; either suppresses other due work
	if coverOutputOperationExpired(runtime, now) {
		immediateOff := runtime.phase == coverActivating || runtime.phase == coverPreparing
		return faultCover(controller, runtime, fmt.Errorf("cover output operation timed out"), now, immediateOff)
	}
	if runtime.phase == coverMoving && !now.Before(runtime.travelDeadline) {
		return stopCompletedCoverTravel(controller, runtime, now)
	}
	events := retryCoverSwitchOff(controller, runtime, now)
	return append(events, reportCoverMovement(controller, runtime, now)...)
}

func coverOutputOperationExpired(runtime *coverRuntime, now time.Time) bool {
	for _, operation := range runtime.pending {
		if operation.generation == runtime.generation && !now.Before(operation.deadline) {
			return true
		}
	}
	return false
}

func stopCompletedCoverTravel(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	runtime.stopReason = coverCompleted
	runtime.travelDeadline = time.Time{}
	runtime.reportDeadline = time.Time{}
	return commandBothCoverOutputsOff(controller, runtime, coverStopping, now)
}

func retryCoverSwitchOff(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	if runtime.retryAt.IsZero() || now.Before(runtime.retryAt) {
		return nil
	}
	return commandBothCoverOutputsOff(controller, runtime, coverRecovering, now)
}

func reportCoverMovement(controller *coverController, runtime *coverRuntime, now time.Time) []event.Event {
	if runtime.phase != coverMoving || now.Before(runtime.reportDeadline) {
		return nil
	}
	observation := coverObservation(runtime, now, controller.settings.FullTravelDuration, event.CoverObservationKind)
	runtime.reportDeadline = now.Add(controller.settings.ReportingInterval)
	return []event.Event{observation}
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
