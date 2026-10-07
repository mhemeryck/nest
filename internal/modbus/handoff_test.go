package modbus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandoffFIFOCoalescingAndOverflow(t *testing.T) {
	handoff := NewHandoff(2)
	first := WriteCoilCommand(1, 10, true)
	second := WriteCoilCommand(1, 11, true)
	rejected := WriteCoilCommand(1, 12, true)
	require.Nil(t, QueueCommand(handoff, first))
	require.Nil(t, QueueCommand(handoff, second))
	failure := QueueCommand(handoff, rejected)
	require.NotNil(t, failure)
	assert.Equal(t, WriteFailedEventKind, failure.Kind)
	assert.Equal(t, rejected.Coil, failure.Coil)
	assert.Contains(t, failure.Error, "capacity exhausted")
	require.Nil(t, QueueCommand(handoff, SetCoilStateCommand(20, false)))
	require.Nil(t, QueueCommand(handoff, SetCoilStateCommand(20, true)))
	commands := make(chan Command)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go RunHandoff(ctx, handoff, commands, done)
	defer func() { cancel(); <-done }()
	for _, expected := range []Command{first, second, SetCoilStateCommand(20, true)} {
		select {
		case command := <-commands:
			assert.Equal(t, expected, command)
		case <-time.After(time.Second):
			require.FailNow(t, "handoff failed to progress without new input")
		}
	}
	select {
	case command := <-commands:
		require.Failf(t, "unexpected command", "%+v", command)
	case <-time.After(20 * time.Millisecond):
	}
}
