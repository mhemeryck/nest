// Package shade owns the lifecycle of a shade's two direction outputs.
// Hardware execution and request delivery remain separate from the model.
package shade

import (
	"fmt"
	"time"
)

type phase uint8

const (
	recovering phase = iota
	stopped
	preparing
	activating
	moving
	stopping
)

type direction uint8

const (
	noDirection direction = iota
	opening
	closing
)

type request uint8

const (
	openRequest request = iota + 1
	closeRequest
	stopRequest
)

type outputState uint8

const (
	unknown outputState = iota
	off
	on
)

type output struct {
	state   outputState
	retryAt time.Time
}

type write struct {
	direction direction
	on        bool
}

// Normal flow: Stopped -> Preparing -> Activating -> Moving -> Stopping -> Stopped.
// Requests select intent; write completions advance preparation and activation;
// stop or deadline expiry starts cleanup. Errors enter Recovering rather than
// advancing the normal flow. Startup also uses Recovering with unknown outputs.
type lifecycle struct {
	phase         phase
	direction     direction
	deadline      time.Time
	shuttingDown  bool
	outputs       [2]output
	inFlight      write
	writing       bool
	fullTravel    time.Duration
	retryInterval time.Duration
}

// Startup: unknown outputs, no movement intent, OFF cleanup required.
func newLifecycle(fullTravel, retryInterval time.Duration) (lifecycle, error) {
	if fullTravel <= 0 || retryInterval <= 0 {
		return lifecycle{}, fmt.Errorf("shade travel and OFF retry durations must be positive")
	}
	return lifecycle{phase: recovering, fullTravel: fullTravel, retryInterval: retryInterval}, nil
}

// Request dispatch: apply direction behavior or begin stopping.
func handleRequest(state *lifecycle, command request) error {
	switch command {
	case openRequest:
		handleDirectionRequest(state, opening)
	case closeRequest:
		handleDirectionRequest(state, closing)
	case stopRequest:
		beginStopping(state)
	default:
		return fmt.Errorf("unknown shade request: %d", command)
	}
	return nil
}

func handleDirectionRequest(state *lifecycle, requested direction) {
	if state.shuttingDown {
		return
	}

	switch state.phase {
	case stopped:
		prepareMovement(state, requested)
	case preparing, activating, moving:
		if state.direction != requested {
			beginStopping(state)
		}
		// Same direction: preserve intent and the original movement deadline.
	case stopping, recovering:
		// Discard direction requests during OFF cleanup.
	}
}

func prepareMovement(state *lifecycle, requested direction) {
	state.phase = preparing
	state.direction = requested
	// Require fresh successful OFF writes before every activation.
	state.outputs = [2]output{}
}

// The caller starts the returned write before handling its completion.
// Claiming a write is distinct from requesting movement: ON can still be canceled
// while Activating if no write has started. Only one write may be in flight.
func claimNextWrite(state *lifecycle, now time.Time) (write, bool) {
	next, available := selectNextWrite(state, now)
	if !available {
		return write{}, false
	}

	state.inFlight = next
	state.writing = true
	state.outputs[int(next.direction)-1].state = unknown
	return next, true
}

func selectNextWrite(state *lifecycle, now time.Time) (write, bool) {
	if state.writing {
		return write{}, false
	}

	var next write
	var available bool

	switch state.phase {
	case preparing, stopping, recovering:
		next, available = nextOffWrite(state, now)
	case activating:
		next = write{direction: state.direction, on: true}
		available = true
	}

	return next, available
}

func nextOffWrite(state *lifecycle, now time.Time) (write, bool) {
	for _, selected := range []direction{opening, closing} {
		output := state.outputs[int(selected)-1]
		if output.state != off && !now.Before(output.retryAt) {
			return write{direction: selected}, true
		}
	}
	return write{}, false
}

// Completion: record the write result, recover on failure, otherwise advance
// preparation, movement, or cleanup. Returned hardware errors remain visible to
// the caller for logging; they also change the lifecycle to OFF recovery.
func completeWrite(state *lifecycle, writeErr error, now time.Time) error {
	if !state.writing {
		return fmt.Errorf("shade write completed without an in-progress write")
	}
	completed := state.inFlight
	state.writing = false
	state.inFlight = write{}
	output := &state.outputs[int(completed.direction)-1]

	if writeErr != nil {
		output.state = unknown
		beginRecovery(state)
		if !completed.on {
			output.retryAt = now.Add(state.retryInterval)
		}
		return fmt.Errorf("shade direction %d write ON=%t: %w", completed.direction, completed.on, writeErr)
	}

	output.retryAt = time.Time{}
	if completed.on {
		output.state = on
		if state.phase == activating {
			state.phase = moving
			state.deadline = now.Add(state.fullTravel)
		}
		// Canceled intent stays canceled; cleanup follows a late ON result.
		return nil
	}

	output.state = off
	if bothOutputsOff(state) {
		if state.phase == preparing {
			state.phase = activating
		} else {
			finishStopping(state)
		}
	}
	return nil
}

func expireMovement(state *lifecycle, now time.Time) {
	if state.phase == moving && !now.Before(state.deadline) {
		beginStopping(state)
	}
}

func beginStopping(state *lifecycle) {
	switch state.phase {
	case stopped:
		// Both outputs already confirmed OFF.
	case stopping:
		// Continue existing cleanup.
	case recovering:
		// Preserve OFF recovery and retry timing.
	case preparing, activating, moving:
		cancelMovement(state)
		state.phase = stopping
		if !state.writing && bothOutputsOff(state) {
			finishStopping(state)
		}
	}
}

func beginRecovery(state *lifecycle) {
	cancelMovement(state)
	state.phase = recovering
}

func beginShutdown(state *lifecycle) {
	state.shuttingDown = true
	beginStopping(state)
}

func cancelMovement(state *lifecycle) {
	state.direction = noDirection
	state.deadline = time.Time{}
}

func bothOutputsOff(state *lifecycle) bool {
	return state.outputs[0].state == off && state.outputs[1].state == off
}

func finishStopping(state *lifecycle) {
	state.phase = stopped
	cancelMovement(state)
}
