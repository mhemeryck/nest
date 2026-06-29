package nest

import (
	"log/slog"

	"github.com/mhemeryck/nest/internal/entity"
)

func logModbusConfig(root *entity.Root) {
	if root.Modbus.Mode == "" {
		return
	}

	slog.Info(
		"modbus route intents configured",
		"mode", root.Modbus.Mode,
		"event_signals", len(root.Modbus.EventSignals),
		"state_points", len(root.Modbus.StatePoints),
		"event_signal_writes", len(root.Modbus.EventSignalWrites),
		"state_polls", len(root.Modbus.StatePolls),
	)
}
