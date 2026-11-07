// Package cover models movement decisions without performing hardware I/O.
package cover

import (
	"fmt"
	"time"
)

type Movement uint8

const (
	Stopped Movement = iota
	Opening
	Closing
)

type Input uint8

const (
	Open Input = iota + 1
	Close
	Stop
	// CheckDeadline asks the FSM to evaluate its current deadline after a timer notification.
	CheckDeadline
)

// State describes movement intent, not confirmed relay or motor state.
// Stopped has no deadline; Opening and Closing always have one.
type State struct {
	Movement Movement
	Deadline time.Time
}

// Effect describes a coordinated output request, never two independent ON flags.
// Runtime integration must enforce output ordering and handle hardware failures.
type Effect uint8

const (
	NoEffect Effect = iota
	StartOpening
	StartClosing
	StopMovement
)

type Result struct {
	State  State
	Effect Effect
}

// Transition applies one input with an explicit clock and maximum run duration.
// The returned deadline drives runtime scheduling; Stopped cancels that deadline.
// Expiry takes precedence over requests and never queues another movement.
// Invalid arguments leave state unchanged and return no output effect.
func Transition(state State, input Input, now time.Time, maxRun time.Duration) (Result, error) {
	unchanged := Result{State: state}
	if err := validate(state, input, now, maxRun); err != nil {
		return unchanged, err
	}

	// At or past the deadline, stop before handling any request.
	// One shared expiry rule keeps every movement branch subject to the same limit.
	if state.Movement != Stopped && !now.Before(state.Deadline) {
		return Result{State: State{Movement: Stopped}, Effect: StopMovement}, nil
	}

	switch state.Movement {
	case Stopped:
		switch input {
		case Open:
			return Result{
				State:  State{Movement: Opening, Deadline: now.Add(maxRun)},
				Effect: StartOpening,
			}, nil
		case Close:
			return Result{
				State:  State{Movement: Closing, Deadline: now.Add(maxRun)},
				Effect: StartClosing,
			}, nil
		case Stop:
			return Result{State: State{Movement: Stopped}, Effect: StopMovement}, nil
		case CheckDeadline:
			// No active movement; ignore notifications from a canceled deadline.
			return unchanged, nil
		}
	case Opening:
		switch input {
		case Close, Stop:
			return Result{State: State{Movement: Stopped}, Effect: StopMovement}, nil
		case Open, CheckDeadline:
			return unchanged, nil
		}
	case Closing:
		switch input {
		case Open, Stop:
			return Result{State: State{Movement: Stopped}, Effect: StopMovement}, nil
		case Close, CheckDeadline:
			return unchanged, nil
		}
	}

	return unchanged, fmt.Errorf("unhandled cover transition: movement %d, input %d", state.Movement, input)
}

func validate(state State, input Input, now time.Time, maxRun time.Duration) error {
	switch state.Movement {
	case Stopped:
		if !state.Deadline.IsZero() {
			return fmt.Errorf("stopped cover has a movement deadline")
		}
	case Opening, Closing:
		if state.Deadline.IsZero() {
			return fmt.Errorf("moving cover has no movement deadline")
		}
	default:
		return fmt.Errorf("invalid cover movement: %d", state.Movement)
	}

	switch input {
	case Open, Close, Stop, CheckDeadline:
	default:
		return fmt.Errorf("invalid cover input: %d", input)
	}
	if now.IsZero() {
		return fmt.Errorf("cover transition requires a nonzero time")
	}
	if maxRun <= 0 {
		return fmt.Errorf("cover maximum run duration must be positive")
	}
	return nil
}
