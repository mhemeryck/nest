package sysfs

import "time"

type WorkerConfig struct {
	Interval time.Duration
	Devices  []*Device
}

func buildWorkerConfigs(devices []*Device) []WorkerConfig {
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
