package sysfs

import (
	"sync"
	"time"
)

type WorkerConfig struct {
	Interval time.Duration
	Devices  []*Device
}

type PollEvent struct {
	Device   Device
	OldValue Value
	NewValue Value
	IsRising bool
}

func StartWorkers(configs []WorkerConfig) ([]chan struct{}, <-chan PollEvent) {
	stopChs := make([]chan struct{}, len(configs))
	events := make(chan PollEvent, 32)
	var wg sync.WaitGroup

	for i, cfg := range configs {
		stopCh := make(chan struct{})
		stopChs[i] = stopCh

		wg.Add(1)
		go func(cfg WorkerConfig, stopCh chan struct{}) {
			defer wg.Done()
			pollWorker(cfg, stopCh, events)
		}(cfg, stopCh)
	}

	go func() {
		wg.Wait()
		close(events)
	}()

	return stopChs, events
}

func StopWorkers(stopChs []chan struct{}) {
	for _, ch := range stopChs {
		close(ch)
	}
}

func pollWorker(
	cfg WorkerConfig,
	stopCh chan struct{},
	events chan<- PollEvent,
) {
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			for _, device := range cfg.Devices {
				changed, oldValue, err := ReadDevice(device)
				if err != nil {
					continue
				}
				if changed {
					events <- PollEvent{
						Device:   *device,
						OldValue: oldValue,
						NewValue: device.Value,
						IsRising: oldValue == Off && device.Value == On,
					}
				}
			}
		}
	}
}

func BuildWorkerConfigs(devices []*Device) []WorkerConfig {
	configs := make(map[DeviceType]WorkerConfig)

	defaultIntervals := map[DeviceType]time.Duration{
		DigitalInput:  20 * time.Millisecond,
		DigitalOutput: 100 * time.Millisecond,
		RelayOutput:   1 * time.Second,
	}

	for _, d := range devices {
		cfg, ok := configs[d.Type]
		if !ok {
			cfg = WorkerConfig{Interval: defaultIntervals[d.Type]}
		}
		cfg.Devices = append(cfg.Devices, d)
		configs[d.Type] = cfg
	}

	result := make([]WorkerConfig, 0, len(configs))
	for _, cfg := range configs {
		result = append(result, cfg)
	}
	return result
}
