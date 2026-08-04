package modbus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/tarm/serial"
	modbusone "github.com/xiegeo/modbusone"
)

type CommandKind string

const (
	WriteCoilCommandKind CommandKind = "write_coil"
	ReadCoilCommandKind  CommandKind = "read_coil"
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

type EventKind string

const (
	WriteSucceededEventKind EventKind = "write_succeeded"
	WriteFailedEventKind    EventKind = "write_failed"
	CoilReadEventKind       EventKind = "coil_read"
	ReadFailedEventKind     EventKind = "read_failed"
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

	if cfg.Mode != entity.ModbusModeMaster {
		slog.Error("modbus runtime only supports master mode", "mode", cfg.Mode)
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

	runMaster(ctx, modbusone.NewSerialContext(connection, int64(cfg.BaudRate)), cfg.Timeout, commands, events)
}

func runMaster(
	ctx context.Context,
	connection modbusone.SerialContext,
	timeout time.Duration,
	commands <-chan Command,
	events chan<- Event,
) {
	client := modbusone.NewRTUClient(connection, 1)
	if timeout > 0 {
		client.SetServerProcessingTime(timeout)
	}

	var current Command
	var readValue bool
	handler := &modbusone.SimpleHandler{
		ReadCoils: func(_ uint16, _ uint16) ([]bool, error) {
			if current.Kind != WriteCoilCommandKind {
				return nil, fmt.Errorf("unexpected client write callback")
			}
			return []bool{current.Value}, nil
		},
		WriteCoils: func(_ uint16, values []bool) error {
			if current.Kind != ReadCoilCommandKind || len(values) != 1 {
				return fmt.Errorf("unexpected client coil response")
			}
			readValue = values[0]
			return nil
		},
	}

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- client.Serve(handler)
	}()
	defer func() {
		if err := client.Close(); err != nil {
			slog.Error("close modbus client", "error", err)
		}
		<-serveDone
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-serveDone:
			if err != nil {
				slog.Error("modbus client stopped", "error", err)
			}
			return
		case command, ok := <-commands:
			if !ok {
				return
			}

			current = command
			readValue = false
			event := executeCommand(client, command)
			if command.Kind == ReadCoilCommandKind && event.Kind == CoilReadEventKind {
				event.Value = readValue
			}

			select {
			case <-ctx.Done():
				return
			case events <- event:
			}
		}
	}
}

func executeCommand(client *modbusone.RTUClient, command Command) Event {
	if command.UnitID == 0 || command.UnitID > 247 {
		return failedEvent(command, fmt.Errorf("unit id must be between 1 and 247"))
	}

	pdu, err := commandPDU(command)
	if err != nil {
		return failedEvent(command, err)
	}
	if err := modbusone.DoRTUTransaction(client, modbusone.RTUHeader{SlaveID: command.UnitID, PDU: pdu}); err != nil {
		return failedEvent(command, err)
	}

	if command.Kind == ReadCoilCommandKind {
		return Event{Kind: CoilReadEventKind, UnitID: command.UnitID, Coil: command.Coil}
	}

	return Event{Kind: WriteSucceededEventKind, UnitID: command.UnitID, Coil: command.Coil, Value: command.Value}
}

func commandPDU(command Command) (modbusone.PDU, error) {
	switch command.Kind {
	case WriteCoilCommandKind:
		return modbusone.FcWriteSingleCoil.MakeRequestHeader(command.Coil, 1)
	case ReadCoilCommandKind:
		return modbusone.FcReadCoils.MakeRequestHeader(command.Coil, 1)
	default:
		return nil, fmt.Errorf("unsupported command kind %q", command.Kind)
	}
}

func failedEvent(command Command, err error) Event {
	kind := WriteFailedEventKind
	if command.Kind == ReadCoilCommandKind {
		kind = ReadFailedEventKind
	}

	return Event{Kind: kind, UnitID: command.UnitID, Coil: command.Coil, Error: err.Error()}
}
