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
	if message.Retain {
		if _, known := handoff.latest[message.Topic]; !known && len(handoff.latest) >= handoff.capacity {
			return fmt.Errorf("mqtt observation handoff capacity exhausted")
		}
		handoff.latest[message.Topic] = command
		handoff.pending[message.Topic] = command
	} else {
		if len(handoff.events) >= handoff.capacity {
			return fmt.Errorf("mqtt event handoff capacity exhausted")
		}
		handoff.events = append(handoff.events, command)
	}
	select {
	case handoff.wake <- struct{}{}:
	default:
	}
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
	for len(handoff.events) > 0 {
		select {
		case commands <- handoff.events[0]:
			handoff.events[0] = Command{}
			handoff.events = handoff.events[1:]
		default:
			return true
		}
	}
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
