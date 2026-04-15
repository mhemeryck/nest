package sysfs

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
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

type PollHandler struct {
	mu      sync.Mutex
	OnEvent func(PollEvent)
}

func (h *PollHandler) Emit(event PollEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.OnEvent != nil {
		h.OnEvent(event)
	}
}

func StartWorkers(
	configs []WorkerConfig,
	onEvent func(PollEvent),
) []chan struct{} {
	stopChs := make([]chan struct{}, len(configs))
	handler := &PollHandler{OnEvent: onEvent}

	for i, cfg := range configs {
		stopCh := make(chan struct{})
		stopChs[i] = stopCh

		go pollWorker(cfg, stopCh, handler.Emit)
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
				val, err := ReadFileValue(path)
				if err != nil {
					continue
				}
				v, _ := strconv.Atoi(strings.TrimSpace(val))

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
	{Type: DigitalInput, Regex: regexp.MustCompile(`/di_(?P<group>\d+)_(?P<num>\d+)/di_value$`)},
	{Type: DigitalOutput, Regex: regexp.MustCompile(`/do_(?P<group>\d+)_(?P<num>\d+)/do_value$`)},
	{Type: RelayOutput, Regex: regexp.MustCompile(`/ro_(?P<group>\d+)_(?P<num>\d+)/ro_value$`)},
}

type MatchedDevice struct {
	Path  string
	Type  DeviceType
	Group string
	Num   string
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

func ReadDIValue(devicePath string) (int, error) {
	return ReadGPIOValueFromPath(fmt.Sprintf("%s/di_value", devicePath))
}

func ReadDOValue(devicePath string) (int, error) {
	return ReadGPIOValueFromPath(fmt.Sprintf("%s/do_value", devicePath))
}

func ReadROValue(devicePath string) (int, error) {
	return ReadGPIOValueFromPath(fmt.Sprintf("%s/ro_value", devicePath))
}

func ReadGPIOValueFromPath(path string) (int, error) {
	val, err := ReadFileValue(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(val))
}

func WriteDOValue(devicePath string, value int) error {
	return WriteFileValue(fmt.Sprintf("%s/do_value", devicePath), fmt.Sprintf("%d", value))
}

func WriteROValue(devicePath string, value int) error {
	return WriteFileValue(fmt.Sprintf("%s/ro_value", devicePath), fmt.Sprintf("%d", value))
}
