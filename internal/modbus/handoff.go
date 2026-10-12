package modbus

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Handoff struct {
	mu       sync.Mutex
	capacity int
	events   []Command
	states   map[uint16]Command
	wake     chan struct{}
}

func NewHandoff(capacity int) *Handoff {
	return &Handoff{capacity: capacity, states: make(map[uint16]Command), wake: make(chan struct{}, 1)}
}

func QueueCommand(handoff *Handoff, command Command) *Event {
	handoff.mu.Lock()
	defer handoff.mu.Unlock()
	var failure *Event
	if command.Kind == SetCoilStateCommandKind {
		failure = queueLatestState(handoff, command)
	} else {
		failure = queueEventCommand(handoff, command)
	}
	if failure != nil {
		return failure
	}
	wakeHandoff(handoff)
	return nil
}

func wakeHandoff(handoff *Handoff) {
	select {
	case handoff.wake <- struct{}{}:
	default:
	}
}

// Caller holds the handoff lock across admission and wake-up
func queueLatestState(handoff *Handoff, command Command) *Event {
	if _, exists := handoff.states[command.Coil]; !exists && len(handoff.states) >= handoff.capacity {
		return rejectedCommand(command, "state handoff capacity exhausted")
	}
	handoff.states[command.Coil] = command
	return nil
}

func queueEventCommand(handoff *Handoff, command Command) *Event {
	if len(handoff.events) >= handoff.capacity {
		return rejectedCommand(command, "event handoff capacity exhausted")
	}
	handoff.events = append(handoff.events, command)
	return nil
}

func rejectedCommand(command Command, reason string) *Event {
	return &Event{Kind: WriteFailedEventKind, UnitID: command.UnitID, Coil: command.Coil, Value: command.Value,
		Error: fmt.Sprintf("modbus command rejected: %s", reason)}
}

func RunHandoff(ctx context.Context, handoff *Handoff, commands chan<- Command, done chan<- struct{}) {
	defer close(done)
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		pending := FlushAvailable(handoff, commands)
		if !waitForHandoffWork(ctx, handoff, timer, pending) {
			return
		}
	}
}

func waitForHandoffWork(ctx context.Context, handoff *Handoff, timer *time.Timer, pending bool) bool {
	retry := scheduleHandoffRetry(timer, pending)
	select {
	case <-ctx.Done():
		return false
	case <-handoff.wake:
	case <-retry:
	}
	return true
}

func scheduleHandoffRetry(timer *time.Timer, pending bool) <-chan time.Time {
	if !pending {
		return nil
	}
	timer.Reset(10 * time.Millisecond)
	return timer.C
}

func FlushAvailable(handoff *Handoff, commands chan<- Command) bool {
	handoff.mu.Lock()
	defer handoff.mu.Unlock()
	if flushEventCommands(handoff, commands) {
		return true
	}
	return flushPendingStates(handoff, commands)
}

// Caller holds the handoff lock; FIFO events precede coalesced states
func flushEventCommands(handoff *Handoff, commands chan<- Command) bool {
	for len(handoff.events) > 0 {
		select {
		case commands <- handoff.events[0]:
			handoff.events[0] = Command{}
			handoff.events = handoff.events[1:]
		default:
			return true
		}
	}
	return false
}

func flushPendingStates(handoff *Handoff, commands chan<- Command) bool {
	for coil, command := range handoff.states {
		select {
		case commands <- command:
			delete(handoff.states, coil)
		default:
			return true
		}
	}
	return false
}
