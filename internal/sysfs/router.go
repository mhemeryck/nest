package sysfs

import (
	"context"
	"sync"
)

type workerSet struct {
	routes map[string]chan Command
	wg     sync.WaitGroup
}

func startWorkers(ctx context.Context, configs []WorkerConfig, commands <-chan Command, states chan<- StateChange) <-chan struct{} {
	doneCh := make(chan struct{})

	workers := startConfiguredWorkers(ctx, configs, states)

	go routeCommands(ctx, commands, workers.routes)

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
