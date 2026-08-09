package modbus

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	modbusone "github.com/xiegeo/modbusone"
)

func TestRunMasterExchangesCoilsWithInMemorySlave(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	t.Cleanup(func() {
		assert.NoError(t, clientConnection.Close())
		assert.NoError(t, serverConnection.Close())
	})

	coils := make([]bool, 13)
	server := modbusone.NewRTUServer(modbusone.NewSerialContext(serverConnection, 19200), 2)
	serverHandler := &modbusone.SimpleHandler{
		ReadCoils: func(address, quantity uint16) ([]bool, error) {
			return append([]bool(nil), coils[address:address+quantity]...), nil
		},
		WriteCoils: func(address uint16, values []bool) error {
			copy(coils[address:], values)
			return nil
		},
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(serverHandler) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	commands := make(chan Command, 2)
	events := make(chan Event, 2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runMaster(ctx, modbusone.NewSerialContext(clientConnection, 19200), entity.Modbus{
			Timeout: time.Second,
			EventSignalWrites: []entity.ModbusEventSignalWrite{{
				UnitID: 2,
			}},
		}, commands, events)
	}()

	commands <- WriteCoilCommand(2, 12, true)
	writeEvent := receiveEvent(t, events)
	assert.Equal(t, WriteSucceededEventKind, writeEvent.Kind)
	assert.True(t, writeEvent.Value)
	assert.True(t, coils[12])

	commands <- ReadCoilCommand(2, 12)
	readEvent := receiveEvent(t, events)
	assert.Equal(t, CoilReadEventKind, readEvent.Kind)
	assert.True(t, readEvent.Value)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for master shutdown")
	}

	<-serverDone
}

func TestRunSlaveExchangesConfiguredCoilsWithMaster(t *testing.T) {
	masterConnection, slaveConnection := net.Pipe()
	t.Cleanup(func() {
		assert.NoError(t, masterConnection.Close())
		assert.NoError(t, slaveConnection.Close())
	})

	slaveContext, cancelSlave := context.WithCancel(context.Background())
	t.Cleanup(cancelSlave)
	slaveCommands := make(chan Command, 1)
	slaveEvents := make(chan Event, 2)
	slaveDone := make(chan struct{})
	go func() {
		defer close(slaveDone)
		runSlave(slaveContext, modbusone.NewSerialContext(slaveConnection, 19200), entity.Modbus{
			Mode:   entity.ModbusModeSlave,
			UnitID: 1,
			EventSignals: []entity.ModbusEventSignal{
				{ID: "button_toggle", Coil: 3},
			},
			StatePoints: []entity.ModbusStatePoint{
				{ID: "light_state", Coil: 7},
			},
		}, slaveCommands, slaveEvents)
	}()

	masterContext, cancelMaster := context.WithCancel(context.Background())
	defer cancelMaster()
	masterCommands := make(chan Command, 2)
	masterEvents := make(chan Event, 2)
	masterDone := make(chan struct{})
	go func() {
		defer close(masterDone)
		runMaster(masterContext, modbusone.NewSerialContext(masterConnection, 19200), entity.Modbus{
			Timeout: time.Second,
			EventSignalWrites: []entity.ModbusEventSignalWrite{{
				UnitID: 1,
			}},
		}, masterCommands, masterEvents)
	}()

	masterCommands <- WriteCoilCommand(1, 3, true)
	writeEvent := receiveEvent(t, masterEvents)
	assert.Equal(t, WriteSucceededEventKind, writeEvent.Kind)
	assert.True(t, writeEvent.Value)

	slaveEvent := receiveEvent(t, slaveEvents)
	assert.Equal(t, WriteSucceededEventKind, slaveEvent.Kind)
	assert.Equal(t, uint16(3), slaveEvent.Coil)
	assert.True(t, slaveEvent.Value)

	slaveCommands <- SetCoilStateCommand(7, true)
	stateEvent := receiveEvent(t, slaveEvents)
	assert.Equal(t, StateUpdatedEventKind, stateEvent.Kind)
	assert.Equal(t, uint16(7), stateEvent.Coil)
	assert.True(t, stateEvent.Value)

	masterCommands <- ReadCoilCommand(1, 7)
	readEvent := receiveEvent(t, masterEvents)
	assert.Equal(t, CoilReadEventKind, readEvent.Kind)
	assert.True(t, readEvent.Value)

	cancelMaster()
	select {
	case <-masterDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for master shutdown")
	}

	cancelSlave()
	select {
	case <-slaveDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for slave shutdown")
	}
}

func TestRunMasterPollsConfiguredStatePoints(t *testing.T) {
	clientConnection, serverConnection := net.Pipe()
	t.Cleanup(func() {
		assert.NoError(t, clientConnection.Close())
		assert.NoError(t, serverConnection.Close())
	})

	reads := make(chan struct{}, 2)
	server := modbusone.NewRTUServer(modbusone.NewSerialContext(serverConnection, 19200), 1)
	serverHandler := &modbusone.SimpleHandler{
		ReadCoils: func(address, quantity uint16) ([]bool, error) {
			assert.Equal(t, uint16(7), address)
			assert.Equal(t, uint16(1), quantity)
			select {
			case reads <- struct{}{}:
			default:
			}
			return []bool{true}, nil
		},
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(serverHandler) }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Event, 2)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runMaster(ctx, modbusone.NewSerialContext(clientConnection, 19200), entity.Modbus{
			Timeout:      time.Second,
			PollInterval: 10 * time.Millisecond,
			StatePolls: []entity.ModbusStatePoll{{
				UnitID: 1,
				Coil:   7,
			}},
		}, nil, events)
	}()

	<-reads
	pollEvent := receiveEvent(t, events)
	assert.Equal(t, Event{Kind: CoilReadEventKind, UnitID: 1, Coil: 7, Value: true}, pollEvent)
	<-reads
	assert.Empty(t, events)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for master shutdown")
	}

	<-serverDone
}

func TestCommandPDU(t *testing.T) {
	writePDU, err := commandPDU(WriteCoilCommand(1, 12, true))
	require.NoError(t, err)
	assert.Equal(t, modbusone.PDU{byte(modbusone.FcWriteSingleCoil), 0, 12}, writePDU)

	readPDU, err := commandPDU(ReadCoilCommand(1, 12))
	require.NoError(t, err)
	assert.Equal(t, modbusone.PDU{byte(modbusone.FcReadCoils), 0, 12, 0, 1}, readPDU)
}

func receiveEvent(t *testing.T, events <-chan Event) Event {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Modbus event")
		return Event{}
	}
}
