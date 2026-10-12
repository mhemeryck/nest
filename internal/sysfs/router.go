package sysfs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type workerSet struct {
	routes map[string]chan Command
	wg     sync.WaitGroup
}

func startWorkers(ctx context.Context, configs []WorkerConfig, commands <-chan Command, states chan<- StateChange) <-chan struct{} {
	workers := startConfiguredWorkers(ctx, configs, states)
	startCommandRouter(ctx, workers, commands, states)
	return signalWorkerCompletion(workers)
}

func startCommandRouter(ctx context.Context, workers *workerSet, commands <-chan Command, states chan<- StateChange) {
	workers.wg.Add(1)
	go func() {
		defer workers.wg.Done()
		routeCommands(ctx, commands, workers.routes, states)
	}()
}

func signalWorkerCompletion(workers *workerSet) <-chan struct{} {
	doneCh := make(chan struct{})
	go func() {
		workers.wg.Wait()
		close(doneCh)
	}()

	return doneCh
}

func startConfiguredWorkers(ctx context.Context, configs []WorkerConfig, states chan<- StateChange) *workerSet {
	workers := &workerSet{
		routes: make(map[string]chan Command),
	}

	for _, cfg := range configs {
		startConfiguredWorker(ctx, workers, cfg, states)
	}

	return workers
}

func startConfiguredWorker(ctx context.Context, workers *workerSet, cfg WorkerConfig, states chan<- StateChange) {
	commandCh := make(chan Command, 32)
	registerWorkerDevices(workers.routes, cfg.Devices, commandCh)
	workers.wg.Add(1)
	go func() {
		defer workers.wg.Done()
		if !publishInitialRelayStates(ctx, cfg.Devices, states) {
			return
		}
		pollWorker(ctx, cfg, commandCh, states)
	}()
}

func registerWorkerDevices(routes map[string]chan Command, devices []*Device, commandCh chan Command) {
	for _, device := range devices {
		routes[device.Identifier] = commandCh
	}
}

func routeCommands(ctx context.Context, commands <-chan Command, routes map[string]chan Command, states chan<- StateChange) {
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-commands:
			if !ok {
				return
			}

			if !routeCommand(ctx, cmd, routes, states) {
				return
			}
		}
	}
}

func routeCommand(ctx context.Context, cmd Command, routes map[string]chan Command, states chan<- StateChange) bool {
	commandCh, found := routes[cmd.DeviceID]
	if !found {
		publishCompletion(ctx, states, cmd, time.Now(), fmt.Errorf("unknown device %q", cmd.DeviceID))
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case commandCh <- cmd:
	default:
		publishCompletion(ctx, states, cmd, time.Now(), fmt.Errorf("sysfs worker queue exhausted for device %q", cmd.DeviceID))
	}
	return true
}
