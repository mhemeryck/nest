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

func (state *slaveState) readCoils(address, quantity uint16) ([]bool, error) {
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

func (state *slaveState) writeEventSignals(address uint16, values []bool) ([]Event, error) {
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

func (state *slaveState) setStatePoint(coil uint16, value bool) error {
	state.mu.Lock()
	defer state.mu.Unlock()

	if _, ok := state.statePoints[coil]; !ok {
		return fmt.Errorf("coil address %d is not a configured state point", coil)
	}
	state.coils[coil] = value
	return nil
}

func runSlave(
	ctx context.Context,
	connection modbusone.SerialContext,
	cfg entity.Modbus,
	commands <-chan Command,
	events chan<- Event,
) {
	if cfg.UnitID < 1 || cfg.UnitID > 247 {
		slog.Error("modbus slave unit id must be between 1 and 247", "unit_id", cfg.UnitID)
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
	handler := &modbusone.SimpleHandler{
		ReadCoils: state.readCoils,
		WriteCoils: func(address uint16, values []bool) error {
			writeEvents, err := state.writeEventSignals(address, values)
			if err != nil {
				return err
			}
			for _, event := range writeEvents {
				event.UnitID = uint8(cfg.UnitID)
				if !sendEvent(ctx, events, event) {
					return context.Canceled
				}
			}
			return nil
		},
	}

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
			if command.Kind != SetCoilStateCommandKind {
				if !sendEvent(ctx, events, failedEvent(command, fmt.Errorf("command %q is not supported in slave mode", command.Kind))) {
					stop()
					return
				}
				continue
			}
			if err := state.setStatePoint(command.Coil, command.Value); err != nil {
				if !sendEvent(ctx, events, Event{Kind: WriteFailedEventKind, UnitID: uint8(cfg.UnitID), Coil: command.Coil, Error: err.Error()}) {
					stop()
					return
				}
				continue
			}
			if !sendEvent(ctx, events, Event{Kind: StateUpdatedEventKind, UnitID: uint8(cfg.UnitID), Coil: command.Coil, Value: command.Value}) {
				stop()
				return
			}
		}
	}
}
