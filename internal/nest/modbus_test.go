package nest

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
)

func TestLogModbusConfigLogsConfiguredRouteIntents(t *testing.T) {
	var buffer bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	logModbusConfig(registry.Build(&entity.Root{
		Modbus: entity.Modbus{
			Mode:              entity.ModbusModeMaster,
			EventSignalWrites: []entity.ModbusEventSignalWrite{{}},
			StatePolls:        []entity.ModbusStatePoll{{}},
		},
	}))

	log := buffer.String()
	assert.Contains(t, log, "modbus route intents configured")
	assert.Contains(t, log, "mode=master")
	assert.Contains(t, log, "event_signal_writes=1")
	assert.Contains(t, log, "state_polls=1")
}

func TestLogModbusConfigSkipsUnconfiguredModbus(t *testing.T) {
	var buffer bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	logModbusConfig(registry.Build(&entity.Root{}))

	assert.Empty(t, buffer.String())
}
