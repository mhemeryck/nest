package mqtt

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"
)

type Handoff struct {
	mu       sync.Mutex
	capacity int
	events   []Command
	pending  map[string]Command
	latest   map[string]Command
	wake     chan struct{}
}

func NewHandoff(capacity int) *Handoff {
	return &Handoff{capacity: capacity, pending: make(map[string]Command), latest: make(map[string]Command), wake: make(chan struct{}, 1)}
}

func QueueMessage(handoff *Handoff, message PublishMessage) error {
	handoff.mu.Lock()
	defer handoff.mu.Unlock()
	message.Payload = bytes.Clone(message.Payload)
	command := PublishCommand(message)
	var err error
	if message.Retain {
		err = queueLatestObservation(handoff, message.Topic, command)
	} else {
		err = queueEventCommand(handoff, command)
	}
	if err != nil {
		return err
	}
	select {
	case handoff.wake <- struct{}{}:
	default:
	}
	return nil
}

// Caller holds the handoff lock across admission and wake-up
func queueLatestObservation(handoff *Handoff, topic string, command Command) error {
	if _, known := handoff.latest[topic]; !known && len(handoff.latest) >= handoff.capacity {
		return fmt.Errorf("mqtt observation handoff capacity exhausted")
	}
	handoff.latest[topic] = command
	handoff.pending[topic] = command
	return nil
}

func queueEventCommand(handoff *Handoff, command Command) error {
	if len(handoff.events) >= handoff.capacity {
		return fmt.Errorf("mqtt event handoff capacity exhausted")
	}
	handoff.events = append(handoff.events, command)
	return nil
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
	if flushEventCommands(handoff, commands) {
		return true
	}
	return flushPendingObservations(handoff, commands)
}

// Caller holds the handoff lock; FIFO events precede coalesced observations
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

func flushPendingObservations(handoff *Handoff, commands chan<- Command) bool {
	for topic, command := range handoff.pending {
		select {
		case commands <- command:
			delete(handoff.pending, topic)
		default:
			return true
		}
	}
	return false
}
