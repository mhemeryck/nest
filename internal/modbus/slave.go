package modbus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mhemeryck/nest/internal/entity"
	modbusone "github.com/xiegeo/modbusone"
)

type slaveState struct {
	mu           sync.RWMutex
	coils        map[uint16]bool
	eventSignals map[uint16]struct{}
	statePoints  map[uint16]struct{}
}

func newSlaveState(cfg entity.Modbus) (*slaveState, error) {
	state := &slaveState{
		coils:        make(map[uint16]bool, len(cfg.EventSignals)+len(cfg.StatePoints)),
		eventSignals: make(map[uint16]struct{}, len(cfg.EventSignals)),
		statePoints:  make(map[uint16]struct{}, len(cfg.StatePoints)),
	}

	for _, signal := range cfg.EventSignals {
		coil, err := modbusCoilAddress(signal.Coil)
		if err != nil {
			return nil, fmt.Errorf("event signal %q: %w", signal.ID, err)
		}
		if _, exists := state.coils[coil]; exists {
			return nil, fmt.Errorf("duplicate Modbus coil address %d", coil)
		}
		state.coils[coil] = false
		state.eventSignals[coil] = struct{}{}
	}

	for _, point := range cfg.StatePoints {
		coil, err := modbusCoilAddress(point.Coil)
		if err != nil {
			return nil, fmt.Errorf("state point %q: %w", point.ID, err)
		}
		if _, exists := state.coils[coil]; exists {
			return nil, fmt.Errorf("duplicate Modbus coil address %d", coil)
		}
		state.coils[coil] = false
		state.statePoints[coil] = struct{}{}
	}

	return state, nil
}

func modbusCoilAddress(coil int) (uint16, error) {
	if coil < 0 || coil > 65535 {
		return 0, fmt.Errorf("coil address %d is outside the Modbus range", coil)
	}
	return uint16(coil), nil
}

func readSlaveCoils(state *slaveState, address, quantity uint16) ([]bool, error) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	values := make([]bool, quantity)
	for offset := range values {
		coil := address + uint16(offset)
		value, ok := state.coils[coil]
		if !ok {
			return nil, fmt.Errorf("coil address %d is not configured", coil)
		}
		values[offset] = value
	}
	return values, nil
}

func writeSlaveEventSignals(state *slaveState, address uint16, values []bool) ([]Event, error) {
	state.mu.Lock()
	defer state.mu.Unlock()

	events := make([]Event, 0, len(values))
	for offset, value := range values {
		coil := address + uint16(offset)
		if _, ok := state.eventSignals[coil]; !ok {
			if _, statePoint := state.statePoints[coil]; statePoint {
				return nil, fmt.Errorf("state point coil %d cannot be written", coil)
			}
			return nil, fmt.Errorf("coil address %d is not a configured event signal", coil)
		}
		state.coils[coil] = value
		events = append(events, Event{
			Kind:  WriteSucceededEventKind,
			Coil:  coil,
			Value: value,
		})
	}
	return events, nil
}

func setSlaveStatePoint(state *slaveState, coil uint16, value bool) error {
	state.mu.Lock()
	defer state.mu.Unlock()

	if _, ok := state.statePoints[coil]; !ok {
		return fmt.Errorf("coil address %d is not a configured state point", coil)
	}
	state.coils[coil] = value
	return nil
}

func newSlaveHandler(
	ctx context.Context,
	unitID uint8,
	state *slaveState,
	events chan<- Event,
) *modbusone.SimpleHandler {
	return &modbusone.SimpleHandler{
		ReadCoils: func(address, quantity uint16) ([]bool, error) {
			return readSlaveCoils(state, address, quantity)
		},
		WriteCoils: func(address uint16, values []bool) error {
			writeEvents, err := writeSlaveEventSignals(state, address, values)
			if err != nil {
				return err
			}
			for _, event := range writeEvents {
				event.UnitID = unitID
				if !sendEvent(ctx, events, event) {
					return context.Canceled
				}
			}
			return nil
		},
	}
}

func handleSlaveCommand(
	ctx context.Context,
	unitID uint8,
	state *slaveState,
	command Command,
	events chan<- Event,
) bool {
	if command.Kind != SetCoilStateCommandKind {
		return sendEvent(ctx, events, failedEvent(command, fmt.Errorf("command %q is not supported in slave mode", command.Kind)))
	}
	if err := setSlaveStatePoint(state, command.Coil, command.Value); err != nil {
		return sendEvent(ctx, events, Event{
			Kind:   WriteFailedEventKind,
			UnitID: unitID,
			Coil:   command.Coil,
			Error:  err.Error(),
		})
	}
	return sendEvent(ctx, events, Event{
		Kind:   StateUpdatedEventKind,
		UnitID: unitID,
		Coil:   command.Coil,
		Value:  command.Value,
	})
}

func serveSlave(
	ctx context.Context,
	server *modbusone.RTUServer,
	handler *modbusone.SimpleHandler,
	unitID uint8,
	state *slaveState,
	commands <-chan Command,
	events chan<- Event,
) {
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(handler) }()

	stop := func() {
		_ = server.Close()
		<-serveDone
	}

	for {
		select {
		case <-ctx.Done():
			stop()
			return
		case err := <-serveDone:
			if err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("modbus slave stopped", "error", err)
			}
			return
		case command, ok := <-commands:
			if !ok {
				stop()
				return
			}
			if !handleSlaveCommand(ctx, unitID, state, command, events) {
				stop()
				return
			}
		}
	}
}

func validateSlaveConfig(cfg entity.Modbus) error {
	if cfg.UnitID < 1 || cfg.UnitID > 247 {
		return fmt.Errorf("modbus slave unit id must be between 1 and 247: %d", cfg.UnitID)
	}
	return nil
}

func runSlave(
	ctx context.Context,
	connection modbusone.SerialContext,
	cfg entity.Modbus,
	commands <-chan Command,
	events chan<- Event,
) {
	if err := validateSlaveConfig(cfg); err != nil {
		slog.Error("invalid modbus slave configuration", "error", err)
		_ = connection.Close()
		return
	}

	state, err := newSlaveState(cfg)
	if err != nil {
		slog.Error("configure modbus slave", "error", err)
		_ = connection.Close()
		return
	}

	server := modbusone.NewRTUServer(connection, byte(cfg.UnitID))
	handler := newSlaveHandler(ctx, uint8(cfg.UnitID), state, events)
	serveSlave(ctx, server, handler, uint8(cfg.UnitID), state, commands, events)
}
