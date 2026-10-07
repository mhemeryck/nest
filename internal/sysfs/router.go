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
	doneCh := make(chan struct{})

	workers := startConfiguredWorkers(ctx, configs, states)

	workers.wg.Add(1)
	go func() {
		defer workers.wg.Done()
		routeCommands(ctx, commands, workers.routes, states)
	}()

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
		commandCh := make(chan Command, 32)
		registerWorkerDevices(workers.routes, cfg.Devices, commandCh)

		workers.wg.Add(1)
		go func(cfg WorkerConfig, commandCh <-chan Command) {
			defer workers.wg.Done()
			if !publishInitialRelayStates(ctx, cfg.Devices, states) {
				return
			}
			pollWorker(ctx, cfg, commandCh, states)
		}(cfg, commandCh)
	}

	return workers
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

			commandCh, found := routes[cmd.DeviceID]
			if !found {
				publishCompletion(ctx, states, cmd, time.Now(), fmt.Errorf("unknown device %q", cmd.DeviceID))
				continue
			}

			select {
			case <-ctx.Done():
				return
			case commandCh <- cmd:
			default:
				publishCompletion(ctx, states, cmd, time.Now(), fmt.Errorf("sysfs worker queue exhausted for device %q", cmd.DeviceID))
			}
		}
	}
}
