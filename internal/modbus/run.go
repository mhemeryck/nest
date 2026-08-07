package modbus

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

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

	if err := validateConfig(cfg); err != nil {
		slog.Error("invalid modbus configuration", "error", err)
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

	switch cfg.Mode {
	case entity.ModbusModeMaster:
		runMaster(ctx, serialContext, cfg.Timeout, commands, events)
	case entity.ModbusModeSlave:
		runSlave(ctx, serialContext, cfg, commands, events)
	default:
		slog.Warn("unsupported modbus mode", "mode", cfg.Mode)
		return
	}
}

func validateConfig(cfg entity.Modbus) error {
	if !slices.Contains([]entity.ModbusMode{entity.ModbusModeMaster, entity.ModbusModeSlave}, cfg.Mode) {
		return fmt.Errorf("modbus runtime mode is not supported: %q", cfg.Mode)
	}
	if cfg.Port == "" {
		return fmt.Errorf("modbus port is not configured")
	}
	if cfg.BaudRate <= 0 {
		return fmt.Errorf("modbus baud rate must be positive: %d", cfg.BaudRate)
	}
	return nil
}

func sendEvent(ctx context.Context, events chan<- Event, event Event) bool {
	select {
	case events <- event:
		return true
	case <-ctx.Done():
		return false
	}
}
