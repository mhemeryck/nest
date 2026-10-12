package sysfs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdmissionRejectsWithoutEnqueuing(t *testing.T) {
	commands := make(chan Command, 1)
	first := Command{ID: 1, Kind: OnCommand, DeviceID: "relay"}
	second := Command{ID: 2, Kind: OffCommand, DeviceID: "relay"}
	require.Nil(t, AdmitCommand(commands, first))
	failure := AdmitCommand(commands, second)
	require.NotNil(t, failure)
	assert.Equal(t, second, failure.Command)
	assert.Error(t, failure.Error)
	assert.Equal(t, first, <-commands)
	assert.Empty(t, commands)
}

func TestRejectedActivationNeverExecutesLater(t *testing.T) {
	commands := make(chan Command, 1)
	require.Nil(t, AdmitCommand(commands, Command{ID: 1, Kind: OffCommand, DeviceID: "relay"}))
	activation := Command{ID: 2, Kind: OnCommand, DeviceID: "relay"}
	failure := AdmitCommand(commands, activation)
	require.NotNil(t, failure)
	assert.Equal(t, activation, failure.Command)
	assert.Equal(t, OffCommand, (<-commands).Kind)
	assert.Empty(t, commands)
}

func TestSaturatedWorkerDoesNotBlockAnotherRoute(t *testing.T) {
	blocked := make(chan Command, 1)
	available := make(chan Command, 2)
	blocked <- Command{ID: 1, Kind: OnCommand, DeviceID: "blocked"}
	commands := make(chan Command, 3)
	states := make(chan StateChange, 2)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		routeCommands(ctx, commands, map[string]chan Command{"blocked": blocked, "available": available}, states)
	}()
	defer func() { cancel(); <-done }()
	rejected := Command{ID: 2, Kind: OffCommand, DeviceID: "blocked"}
	commands <- rejected
	commands <- Command{ID: 3, Kind: OnCommand, DeviceID: "available"}
	commands <- Command{ID: 4, Kind: OffCommand, DeviceID: "available"}
	select {
	case report := <-states:
		require.Equal(t, CompletionReportKind, report.Kind)
		assert.Equal(t, rejected, report.Completion.Command)
		assert.ErrorContains(t, report.Completion.Error, "queue exhausted")
	case <-time.After(time.Second):
		require.FailNow(t, "missing rejection result")
	}
	for _, id := range []int{3, 4} {
		select {
		case cmd := <-available:
			assert.EqualValues(t, id, cmd.ID)
		case <-time.After(time.Second):
			require.FailNow(t, "unrelated worker did not receive command")
		}
	}
	assert.EqualValues(t, 1, (<-blocked).ID)
	assert.Empty(t, blocked)
}
