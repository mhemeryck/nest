package sysfs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterWorkerDevices(t *testing.T) {
	routes := make(map[string]chan Command)
	commandCh := make(chan Command)

	registerWorkerDevices(routes, []*Device{{Identifier: "di_1_01"}, {Identifier: "ro_1_01"}}, commandCh)

	assert.Equal(t, commandCh, routes["di_1_01"])
	assert.Equal(t, commandCh, routes["ro_1_01"])
}

func TestRouteCommandsForwardsKnownDevice(t *testing.T) {
	commands := make(chan Command, 1)
	workerCommands := make(chan Command, 1)
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())

	go func() {
		defer close(done)
		routeCommands(ctx, commands, map[string]chan Command{"ro_1_01": workerCommands})
	}()

	commands <- Command{Kind: ToggleCommand, DeviceID: "ro_1_01"}

	forwarded := <-workerCommands
	assert.Equal(t, ToggleCommand, forwarded.Kind)
	assert.Equal(t, "ro_1_01", forwarded.DeviceID)

	cancel()
	<-done
}

func TestRouteCommandsIgnoresUnknownDevice(t *testing.T) {
	commands := make(chan Command, 1)
	workerCommands := make(chan Command, 1)
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())

	go func() {
		defer close(done)
		routeCommands(ctx, commands, map[string]chan Command{"ro_1_01": workerCommands})
	}()

	commands <- Command{Kind: ToggleCommand, DeviceID: "missing"}

	cancel()
	<-done

	select {
	case cmd := <-workerCommands:
		require.Failf(t, "unexpected forwarded command", "%+v", cmd)
	default:
	}
}

func TestStartConfiguredWorkersRegistersRoutes(t *testing.T) {
	states := make(chan StateChange, 1)
	ctx, cancel := context.WithCancel(t.Context())
	workers := startConfiguredWorkers(ctx, []WorkerConfig{{
		Interval: 10 * time.Millisecond,
		Devices:  []*Device{{Identifier: "di_1_01"}, {Identifier: "ro_1_01"}},
	}}, states)
	require.NotNil(t, workers)
	require.Contains(t, workers.routes, "di_1_01")
	require.Contains(t, workers.routes, "ro_1_01")

	cancel()
	workers.wg.Wait()
}
