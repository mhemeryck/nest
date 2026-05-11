package sysfs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildWorkerConfigs(t *testing.T) {
	devices := []*Device{
		{Path: "/di1", Type: DigitalInput, Identifier: "di1"},
		{Path: "/di2", Type: DigitalInput, Identifier: "di2"},
		{Path: "/do1", Type: DigitalOutput, Identifier: "do1"},
		{Path: "/ro1", Type: RelayOutput, Identifier: "ro1"},
	}

	configs := buildWorkerConfigs(devices, PollIntervals{})

	assert.Len(t, configs, 3)

	configByType := make(map[DeviceType]WorkerConfig)
	for _, cfg := range configs {
		for _, device := range cfg.Devices {
			switch device.Path {
			case "/di1", "/di2":
				configByType[DigitalInput] = cfg
			case "/do1":
				configByType[DigitalOutput] = cfg
			case "/ro1":
				configByType[RelayOutput] = cfg
			}
		}
	}

	diCfg, ok := configByType[DigitalInput]
	require.True(t, ok)
	assert.Len(t, diCfg.Devices, 2)
	assert.Equal(t, 100*time.Millisecond, diCfg.Interval)

	roCfg, ok := configByType[RelayOutput]
	require.True(t, ok)
	assert.Len(t, roCfg.Devices, 1)
	assert.Equal(t, 1*time.Second, roCfg.Interval)
}

func TestBuildWorkerConfigsUsesPollIntervalOverrides(t *testing.T) {
	devices := []*Device{
		{Path: "/di1", Type: DigitalInput, Identifier: "di1"},
		{Path: "/ro1", Type: RelayOutput, Identifier: "ro1"},
	}

	configs := buildWorkerConfigs(devices, PollIntervals{
		DigitalInput: 250 * time.Millisecond,
		RelayOutput:  2 * time.Second,
	})

	configByType := make(map[DeviceType]WorkerConfig)
	for _, cfg := range configs {
		configByType[cfg.Devices[0].Type] = cfg
	}

	assert.Equal(t, 250*time.Millisecond, configByType[DigitalInput].Interval)
	assert.Equal(t, 2*time.Second, configByType[RelayOutput].Interval)
}
