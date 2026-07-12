package nest

import (
	"log/slog"

	"github.com/mhemeryck/nest/internal/registry"
)

func logModbusConfig(reg *registry.Registry) {
	modbus := registry.Modbus(reg)
	if modbus.Mode == "" {
		return
	}

	slog.Info(
		"modbus route intents configured",
		"mode", modbus.Mode,
		"event_signals", len(modbus.EventSignals),
		"state_points", len(modbus.StatePoints),
		"event_signal_writes", len(modbus.EventSignalWrites),
		"state_polls", len(modbus.StatePolls),
	)
}
