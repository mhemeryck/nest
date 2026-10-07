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
	if command.Kind == SetCoilStateCommandKind {
		if _, exists := handoff.states[command.Coil]; !exists && len(handoff.states) >= handoff.capacity {
			return rejectedCommand(command, "state handoff capacity exhausted")
		}
		handoff.states[command.Coil] = command
	} else {
		if len(handoff.events) >= handoff.capacity {
			return rejectedCommand(command, "event handoff capacity exhausted")
		}
		handoff.events = append(handoff.events, command)
	}
	select {
	case handoff.wake <- struct{}{}:
	default:
	}
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
		var retry <-chan time.Time
		if pending {
			timer.Reset(10 * time.Millisecond)
			retry = timer.C
		}
		select {
		case <-ctx.Done():
			return
		case <-handoff.wake:
		case <-retry:
		}
	}
}

func FlushAvailable(handoff *Handoff, commands chan<- Command) bool {
	handoff.mu.Lock()
	defer handoff.mu.Unlock()
	for len(handoff.events) > 0 {
		select {
		case commands <- handoff.events[0]:
			handoff.events[0] = Command{}
			handoff.events = handoff.events[1:]
		default:
			return true
		}
	}
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
