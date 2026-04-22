package sysfs

import (
	"fmt"
	"sync"
)

type workerSet struct {
	routes map[string]chan Command
	wg     sync.WaitGroup
}

func startWorkers(configs []WorkerConfig, commands <-chan Command, states chan<- PollEvent) (chan<- struct{}, <-chan struct{}) {
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})

	workers := startConfiguredWorkers(configs, stopCh, states)

	go routeCommands(stopCh, commands, workers.routes)

	go func() {
		workers.wg.Wait()
		close(doneCh)
	}()

	return stopCh, doneCh
}

func startConfiguredWorkers(configs []WorkerConfig, stopCh <-chan struct{}, states chan<- PollEvent) *workerSet {
	workers := &workerSet{
		routes: make(map[string]chan Command),
	}

	for _, cfg := range configs {
		commandCh := make(chan Command, 32)
		registerWorkerDevices(workers.routes, cfg.Devices, commandCh)

		workers.wg.Add(1)
		go func(cfg WorkerConfig, commandCh <-chan Command) {
			defer workers.wg.Done()
			pollWorker(cfg, stopCh, commandCh, states)
		}(cfg, commandCh)
	}

	return workers
}

func registerWorkerDevices(routes map[string]chan Command, devices []*Device, commandCh chan Command) {
	for _, device := range devices {
		routes[device.Identifier] = commandCh
	}
}

func routeCommands(stopCh <-chan struct{}, commands <-chan Command, routes map[string]chan Command) {
	for {
		select {
		case <-stopCh:
			return
		case cmd, ok := <-commands:
			if !ok {
				return
			}

			commandCh, found := routes[cmd.DeviceID]
			if !found {
				respondCommand(cmd, CommandResult{DeviceID: cmd.DeviceID, Err: fmt.Errorf("unknown device %q", cmd.DeviceID)})
				continue
			}

			select {
			case <-stopCh:
				respondCommand(cmd, CommandResult{DeviceID: cmd.DeviceID, Err: fmt.Errorf("sysfs workers stopped")})
			case commandCh <- cmd:
			}
		}
	}
}
