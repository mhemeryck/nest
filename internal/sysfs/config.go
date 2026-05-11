package sysfs

import "time"

type WorkerConfig struct {
	Interval time.Duration
	Devices  []*Device
}

type PollIntervals struct {
	DigitalInput  time.Duration
	DigitalOutput time.Duration
	RelayOutput   time.Duration
}

func buildWorkerConfigs(devices []*Device, intervals PollIntervals) []WorkerConfig {
	configs := make(map[DeviceType]WorkerConfig)

	defaultIntervals := defaultPollIntervals(intervals)

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

func defaultPollIntervals(overrides PollIntervals) map[DeviceType]time.Duration {
	intervals := map[DeviceType]time.Duration{
		DigitalInput:  100 * time.Millisecond,
		DigitalOutput: 100 * time.Millisecond,
		RelayOutput:   1 * time.Second,
	}

	if overrides.DigitalInput > 0 {
		intervals[DigitalInput] = overrides.DigitalInput
	}
	if overrides.DigitalOutput > 0 {
		intervals[DigitalOutput] = overrides.DigitalOutput
	}
	if overrides.RelayOutput > 0 {
		intervals[RelayOutput] = overrides.RelayOutput
	}

	return intervals
}
