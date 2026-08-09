package modbus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	modbusone "github.com/xiegeo/modbusone"
)

const defaultPollInterval = time.Second

type masterCoilKey struct {
	unitID uint8
	coil   uint16
}

type masterTransactionState struct {
	current   Command
	readValue bool
}

func runMaster(
	ctx context.Context,
	connection modbusone.SerialContext,
	cfg entity.Modbus,
	commands <-chan Command,
	events chan<- Event,
) {
	state := &masterTransactionState{}
	// Set up the master RTU client that issues reads and writes to slave coils.
	client, serveDone := startMasterClient(connection, cfg, state)
	serveStopped := false
	defer func() {
		if err := client.Close(); err != nil {
			slog.Error("close modbus client", "error", err)
		}
		if !serveStopped {
			<-serveDone
		}
	}()

	ticker := newMasterPollTicker(cfg)
	if ticker != nil {
		defer ticker.Stop()
	}

	// Poll all configured state points once at startup before waiting for the first ticker interval.
	pollValues := make(map[masterCoilKey]bool, len(cfg.StatePolls))
	if !pollSlaveStatePoints(ctx, client, state, cfg.StatePolls, pollValues, events) {
		return
	}

	// runMasterLoop runs the main master-to-slave and slave-to-master commands
	serveStopped = runMasterLoop(ctx, client, state, cfg.StatePolls, pollValues, commands, events, ticker, serveDone)
}

func startMasterClient(
	connection modbusone.SerialContext,
	cfg entity.Modbus,
	state *masterTransactionState,
) (*modbusone.RTUClient, <-chan error) {
	client := modbusone.NewRTUClient(connection, 1)
	if cfg.Timeout > 0 {
		client.SetServerProcessingTime(cfg.Timeout)
	}

	handler := &modbusone.SimpleHandler{
		ReadCoils: func(_ uint16, _ uint16) ([]bool, error) {
			if state.current.Kind != WriteCoilCommandKind {
				return nil, fmt.Errorf("unexpected client write callback")
			}
			return []bool{state.current.Value}, nil
		},
		WriteCoils: func(_ uint16, values []bool) error {
			if state.current.Kind != ReadCoilCommandKind || len(values) != 1 {
				return fmt.Errorf("unexpected client coil response")
			}
			state.readValue = values[0]
			return nil
		},
	}

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- client.ServeRTU(masterHandlers(cfg, handler))
	}()

	return client, serveDone
}

func masterHandlers(cfg entity.Modbus, handler modbusone.ProtocolHandler) modbusone.MultiIDHandler {
	handlers := make(modbusone.MultiIDHandler, len(cfg.EventSignalWrites)+len(cfg.StatePolls))
	for _, write := range cfg.EventSignalWrites {
		handlers[write.UnitID] = handler
	}
	for _, poll := range cfg.StatePolls {
		handlers[poll.UnitID] = handler
	}

	return handlers
}

// runMasterLoop handles:
//   - shutdown: runtime cancellation or client stop
//   - master-to-slave: controller commands
//   - slave-to-master: scheduled coil polling
func runMasterLoop(
	ctx context.Context,
	client *modbusone.RTUClient,
	state *masterTransactionState,
	polls []entity.ModbusStatePoll,
	pollValues map[masterCoilKey]bool,
	commands <-chan Command,
	events chan<- Event,
	ticker *time.Ticker,
	serveDone <-chan error,
) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case err := <-serveDone:
			return masterClientStopped(err)
		case command, ok := <-commands:
			if !ok {
				return false
			}
			if !handleMasterToSlaveCommand(ctx, client, state, command, events) {
				return false
			}
		case <-masterPollTicks(ticker):
			if !pollSlaveStatePoints(ctx, client, state, polls, pollValues, events) {
				return false
			}
		}
	}
}

func masterClientStopped(err error) bool {
	if err != nil {
		slog.Error("modbus client stopped", "error", err)
	}

	return true
}

func newMasterPollTicker(cfg entity.Modbus) *time.Ticker {
	if len(cfg.StatePolls) == 0 {
		return nil
	}

	return time.NewTicker(masterPollInterval(cfg.PollInterval))
}

func masterPollTicks(ticker *time.Ticker) <-chan time.Time {
	if ticker == nil {
		return nil
	}

	return ticker.C
}

func masterPollInterval(configured time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}

	return defaultPollInterval
}

func handleMasterToSlaveCommand(
	ctx context.Context,
	client *modbusone.RTUClient,
	state *masterTransactionState,
	command Command,
	events chan<- Event,
) bool {
	return sendEvent(ctx, events, masterCommandEvent(client, state, command))
}

func pollSlaveStatePoints(
	ctx context.Context,
	client *modbusone.RTUClient,
	state *masterTransactionState,
	polls []entity.ModbusStatePoll,
	pollValues map[masterCoilKey]bool,
	events chan<- Event,
) bool {
	for _, poll := range polls {
		pollEvent := masterCommandEvent(client, state, ReadCoilCommand(poll.UnitID, poll.Coil))
		if pollEvent.Kind == CoilReadEventKind {
			key := masterCoilKey{unitID: poll.UnitID, coil: poll.Coil}
			if previous, observed := pollValues[key]; observed && previous == pollEvent.Value {
				continue
			}
			pollValues[key] = pollEvent.Value
		}
		if !sendEvent(ctx, events, pollEvent) {
			return false
		}
	}

	return true
}

func masterCommandEvent(client *modbusone.RTUClient, state *masterTransactionState, command Command) Event {
	state.current = command
	state.readValue = false
	event := executeCommand(client, command)
	if command.Kind == ReadCoilCommandKind && event.Kind == CoilReadEventKind {
		event.Value = state.readValue
	}

	return event
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
