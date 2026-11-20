package sysfs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunStartsAndStopsWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan Command, 32)
	states := make(chan StateChange, 32)
	done := make(chan struct{})
	go func() {
		assert.NoError(t, Run(ctx, []*Device{{Path: "/tmp/test1", Identifier: "test1"}}, commands, states, done))
	}()

	cancel()
	<-done
	close(states)
}
