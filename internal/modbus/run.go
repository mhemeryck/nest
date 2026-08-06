package modbus

import (
	"context"
	"log/slog"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/tarm/serial"
	modbusone "github.com/xiegeo/modbusone"
)

type CommandKind string

const (
	WriteCoilCommandKind    CommandKind = "write_coil"
	ReadCoilCommandKind     CommandKind = "read_coil"
	SetCoilStateCommandKind CommandKind = "set_coil_state"
)

type Command struct {
	Kind   CommandKind
	UnitID uint8
	Coil   uint16
	Value  bool
}

func WriteCoilCommand(unitID uint8, coil uint16, value bool) Command {
	return Command{Kind: WriteCoilCommandKind, UnitID: unitID, Coil: coil, Value: value}
}

func ReadCoilCommand(unitID uint8, coil uint16) Command {
	return Command{Kind: ReadCoilCommandKind, UnitID: unitID, Coil: coil}
}

func SetCoilStateCommand(coil uint16, value bool) Command {
	return Command{Kind: SetCoilStateCommandKind, Coil: coil, Value: value}
}

type EventKind string

const (
	WriteSucceededEventKind EventKind = "write_succeeded"
	WriteFailedEventKind    EventKind = "write_failed"
	CoilReadEventKind       EventKind = "coil_read"
	ReadFailedEventKind     EventKind = "read_failed"
	StateUpdatedEventKind   EventKind = "state_updated"
)

type Event struct {
	Kind   EventKind
	UnitID uint8
	Coil   uint16
	Value  bool
	Error  string
}

func Run(
	ctx context.Context,
	cfg entity.Modbus,
	commands <-chan Command,
	events chan<- Event,
	done chan<- struct{},
) {
	defer close(done)

	if cfg.Mode != entity.ModbusModeMaster && cfg.Mode != entity.ModbusModeSlave {
		slog.Error("modbus runtime mode is not supported", "mode", cfg.Mode)
		return
	}
	if cfg.Port == "" {
		slog.Error("modbus port is not configured")
		return
	}
	if cfg.BaudRate <= 0 {
		slog.Error("modbus baud rate must be positive", "baud_rate", cfg.BaudRate)
		return
	}

	connection, err := serial.OpenPort(&serial.Config{
		Name:        cfg.Port,
		Baud:        cfg.BaudRate,
		ReadTimeout: cfg.Timeout,
	})
	if err != nil {
		slog.Error("open modbus serial port", "port", cfg.Port, "error", err)
		return
	}

	serialContext := modbusone.NewSerialContext(connection, int64(cfg.BaudRate))
	if cfg.Mode == entity.ModbusModeMaster {
		runMaster(ctx, serialContext, cfg.Timeout, commands, events)
		return
	}

	runSlave(ctx, serialContext, cfg, commands, events)
}

func sendEvent(ctx context.Context, events chan<- Event, event Event) bool {
	select {
	case events <- event:
		return true
	case <-ctx.Done():
		return false
	}
}
