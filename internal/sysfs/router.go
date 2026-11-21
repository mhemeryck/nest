package sysfs

import (
	"context"
	"sync"
)

func startWorkers(ctx context.Context, configs []WorkerConfig, commands <-chan Command, states chan<- StateChange) <-chan struct{} {
	var workers sync.WaitGroup
	routes := make(map[string]chan Command)

	// Register device routes and start their workers.
	for _, cfg := range configs {
		commandCh := make(chan Command, 32)
		registerWorkerDevices(routes, cfg.Devices, commandCh)

		workers.Go(func() {
			if !publishInitialRelayStates(ctx, cfg.Devices, states) {
				return
			}
			pollWorker(ctx, cfg, commandCh, states)
		})
	}

	// Start the router after all routes are registered.
	workers.Go(func() {
		routeCommands(ctx, commands, routes)
	})

	// Signal shutdown after the workers and router exit.
	doneCh := make(chan struct{})
	go func() {
		workers.Wait()
		close(doneCh)
	}()

	return doneCh
}

func registerWorkerDevices(routes map[string]chan Command, devices []*Device, commandCh chan Command) {
	for _, device := range devices {
		routes[device.Identifier] = commandCh
	}
}

func routeCommands(ctx context.Context, commands <-chan Command, routes map[string]chan Command) {
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-commands:
			if !ok {
				return
			}

			commandCh, found := routes[cmd.DeviceID]
			if !found {
				continue
			}

			select {
			case <-ctx.Done():
			case commandCh <- cmd:
			}
		}
	}
}
