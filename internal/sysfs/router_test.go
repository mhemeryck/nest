package sysfs

import (
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
	stopCh := make(chan struct{})
	commands := make(chan Command, 1)
	workerCommands := make(chan Command, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		routeCommands(stopCh, commands, map[string]chan Command{"ro_1_01": workerCommands})
	}()

	commands <- Command{Kind: ToggleCommand, DeviceID: "ro_1_01"}

	forwarded := <-workerCommands
	assert.Equal(t, ToggleCommand, forwarded.Kind)
	assert.Equal(t, "ro_1_01", forwarded.DeviceID)

	close(stopCh)
	<-done
}

func TestRouteCommandsIgnoresUnknownDevice(t *testing.T) {
	stopCh := make(chan struct{})
	commands := make(chan Command, 1)
	workerCommands := make(chan Command, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		routeCommands(stopCh, commands, map[string]chan Command{"ro_1_01": workerCommands})
	}()

	commands <- Command{Kind: ToggleCommand, DeviceID: "missing"}

	close(stopCh)
	<-done

	select {
	case cmd := <-workerCommands:
		t.Fatalf("unexpected forwarded command: %+v", cmd)
	default:
	}
}

func TestStartConfiguredWorkersRegistersRoutes(t *testing.T) {
	stopCh := make(chan struct{})
	states := make(chan PollEvent, 1)
	workers := startConfiguredWorkers([]WorkerConfig{{
		Interval: 10 * time.Millisecond,
		Devices:  []*Device{{Identifier: "di_1_01"}, {Identifier: "ro_1_01"}},
	}}, stopCh, states)
	require.NotNil(t, workers)
	require.Contains(t, workers.routes, "di_1_01")
	require.Contains(t, workers.routes, "ro_1_01")

	close(stopCh)
	workers.wg.Wait()
}
