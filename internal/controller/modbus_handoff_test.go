package controller

import (
	"testing"

	"github.com/mhemeryck/nest/internal/controller/event"
	"github.com/mhemeryck/nest/internal/modbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModbusOverflowReportsLoggedIntegrationFailure(t *testing.T) {
	handoff := modbus.NewHandoff(1)
	command := modbus.WriteCoilCommand(1, 10, true)
	require.Nil(t, modbus.QueueCommand(handoff, command))
	var failures []event.Event
	logs := captureLogs(t, func() {
		failures = submitModbusCommand(t.Context(), make(chan modbus.Command), command, handoff)
	})
	require.Len(t, failures, 1)
	assert.Equal(t, event.IntegrationFailureKind, failures[0].Kind)
	assert.Equal(t, "modbus", failures[0].IntegrationFailure.Integration)
	assert.Contains(t, logs, "modbus integration failure")
}
