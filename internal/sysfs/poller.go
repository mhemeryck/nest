package sysfs

import (
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

type Value byte

const (
	Off Value = '0'
	On  Value = '1'
)

func (v Value) Int() int {
	if v == On {
		return 1
	}

	return 0
}

type WorkerConfig struct {
	Interval time.Duration
	Devices  []*Device
}

type Device struct {
	Type       DeviceType
	Path       string
	Identifier string
	Value      Value
	Ready      bool
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
						Device:    *device,
						OldValue:  oldValue,
						NewValue:  device.Value,
						IsRising:  oldValue == Off && device.Value == On,
					}
				}
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

func MatchDevices(paths []string) []*Device {
	return MatchDevicesWithPatterns(paths, devicePatterns)
}

func MatchDevicesWithPatterns(paths []string, patterns []DevicePattern) []*Device {
	var matched []*Device
	for _, path := range paths {
		for _, p := range patterns {
			if p.Regex.MatchString(path) {
				matched = append(matched, &Device{
					Path:       path,
					Type:       p.Type,
					Identifier: filepath.Base(filepath.Dir(path)),
				})
				break
			}
		}
	}
	return matched
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

func ReadDevice(device *Device) (bool, Value, error) {
	value, err := ReadValue(device.Path)
	if err != nil {
		return false, Off, err
	}
	if !device.Ready {
		device.Value = value
		device.Ready = true

		return false, value, nil
	}

	oldValue := device.Value
	changed := oldValue != value
	device.Value = value

	return changed, oldValue, nil
}

func ReadValue(path string) (Value, error) {
	data, err := ReadFileBytes(path)
	if err != nil {
		return Off, err
	}
	if len(data) == 0 {
		return Off, ErrEmptyValue
	}

	switch data[0] {
	case '0':
		return Off, nil
	case '1':
		return On, nil
	default:
		return Off, ErrInvalidValue
	}
}
