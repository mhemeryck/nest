package sysfs

import "testing"

func TestRunStartsAndStopsWorkers(t *testing.T) {
	commands := make(chan Command, 32)
	states := make(chan PollEvent, 32)
	shutdown := Run([]*Device{{Path: "/tmp/test1", Identifier: "test1"}}, commands, states)

	shutdown()
	close(states)
}
