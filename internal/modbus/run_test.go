package modbus

import (
	"context"
	"net"
	"testing"
	"time"

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
	server := modbusone.NewRTUServer(modbusone.NewSerialContext(serverConnection, 19200), 1)
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
		runMaster(ctx, modbusone.NewSerialContext(clientConnection, 19200), time.Second, commands, events)
	}()

	commands <- WriteCoilCommand(1, 12, true)
	writeEvent := receiveEvent(t, events)
	assert.Equal(t, WriteSucceededEventKind, writeEvent.Kind)
	assert.True(t, writeEvent.Value)
	assert.True(t, coils[12])

	commands <- ReadCoilCommand(1, 12)
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
