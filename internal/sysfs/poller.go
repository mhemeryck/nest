package sysfs

import (
	"errors"
	"regexp"
	"sync"
	"time"
)

type WorkerConfig struct {
	Interval time.Duration
	Paths    []string
}

type WorkerState struct {
	mu         sync.Mutex
	LastValues map[string]int
}

func NewWorkerState() *WorkerState {
	return &WorkerState{LastValues: make(map[string]int)}
}

type PollEvent struct {
	Path     string
	OldValue int
	NewValue int
	IsRising bool
}

func StartWorkers(
	configs []WorkerConfig,
	onEvent func(PollEvent),
) []chan struct{} {
	stopChs := make([]chan struct{}, len(configs))

	for i, cfg := range configs {
		stopCh := make(chan struct{})
		stopChs[i] = stopCh

		go pollWorker(cfg, stopCh, onEvent)
	}

	return stopChs
}

func StopWorkers(stopChs []chan struct{}) {
	for _, ch := range stopChs {
		close(ch)
	}
}

func pollWorker(
	cfg WorkerConfig,
	stopCh chan struct{},
	onEvent func(PollEvent),
) {
	state := NewWorkerState()
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			for _, path := range cfg.Paths {
				v, err := ReadValue(path)
				if err != nil {
					continue
				}

				state.mu.Lock()
				last, ok := state.LastValues[path]
				if ok && v != last {
					onEvent(PollEvent{
						Path:     path,
						OldValue: last,
						NewValue: v,
						IsRising: v == 1 && last == 0,
					})
				}
				state.LastValues[path] = v
				state.mu.Unlock()
			}
		}
	}
}

type DeviceType int

const (
	DigitalInput DeviceType = iota
	DigitalOutput
	RelayOutput
)

func (d DeviceType) String() string {
	switch d {
	case DigitalInput:
		return "di"
	case DigitalOutput:
		return "do"
	case RelayOutput:
		return "ro"
	}
	return ""
}

type DevicePattern struct {
	Type  DeviceType
	Regex *regexp.Regexp
}

var devicePatterns = []DevicePattern{
	{Type: DigitalInput, Regex: regexp.MustCompile(`/di_\d+_\d+/di_value$`)},
	{Type: DigitalOutput, Regex: regexp.MustCompile(`/do_\d+_\d+/do_value$`)},
	{Type: RelayOutput, Regex: regexp.MustCompile(`/ro_\d+_\d+/ro_value$`)},
}

type MatchedDevice struct {
	Path string
	Type DeviceType
}

func MatchDevices(paths []string) []MatchedDevice {
	return MatchDevicesWithPatterns(paths, devicePatterns)
}

func MatchDevicesWithPatterns(paths []string, patterns []DevicePattern) []MatchedDevice {
	var matched []MatchedDevice
	for _, path := range paths {
		for _, p := range patterns {
			if p.Regex.MatchString(path) {
				matched = append(matched, MatchedDevice{
					Path: path,
					Type: p.Type,
				})
				break
			}
		}
	}
	return matched
}

func BuildWorkerConfigs(devices []MatchedDevice) []WorkerConfig {
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
		cfg.Paths = append(cfg.Paths, d.Path)
		configs[d.Type] = cfg
	}

	result := make([]WorkerConfig, 0, len(configs))
	for _, cfg := range configs {
		result = append(result, cfg)
	}
	return result
}

func ReadValue(path string) (int, error) {
	data, err := ReadFileBytes(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, errors.New("empty file")
	}

	switch data[0] {
	case '0':
		return 0, nil
	case '1':
		return 1, nil
	default:
		return 0, errors.New("invalid value")
	}
}
