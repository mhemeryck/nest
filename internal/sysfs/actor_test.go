package sysfs

import (
	"context"
	"testing"
)

func TestRunStartsAndStopsWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan Command, 32)
	states := make(chan PollEvent, 32)
	done := make(chan struct{})
	go Run(ctx, []*Device{{Path: "/tmp/test1", Identifier: "test1"}}, commands, states, done)

	cancel()
	<-done
	close(states)
}
