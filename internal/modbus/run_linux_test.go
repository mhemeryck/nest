//go:build linux

package modbus

import (
	"context"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	modbusone "github.com/xiegeo/modbusone"
)

func TestRunMasterOpensConfiguredPTY(t *testing.T) {
	// Test Modbus peer <-> PTY master <-> kernel PTY <-> slave path <-> Nest master.
	master, slave, err := pty.Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = master.Close()
		_ = slave.Close()
	})

	// Test-owned slave peer; coil values after Nest RTU writes.
	coils := make([]bool, 8)
	server := modbusone.NewRTUServer(modbusone.NewSerialContext(master, 19200), 1)
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- server.Serve(&modbusone.SimpleHandler{
			ReadCoils: func(address, quantity uint16) ([]bool, error) {
				return append([]bool(nil), coils[address:address+quantity]...), nil
			},
			WriteCoils: func(address uint16, values []bool) error {
				copy(coils[address:], values)
				return nil
			},
		})
	}()

	ctx, cancel := context.WithCancel(t.Context())
	commands := make(chan Command, 1)
	events := make(chan Event, 1)
	done := make(chan struct{})
	// Production actor; tarm/serial opens the PTY slave path.
	go Run(ctx, entity.Modbus{
		Mode:     entity.ModbusModeMaster,
		Port:     slave.Name(),
		BaudRate: 19200,
		Timeout:  100 * time.Millisecond,
		EventSignalWrites: []entity.ModbusEventSignalWrite{{
			UnitID: 1,
		}},
	}, commands, events, done)

	// Test -> command channel -> Nest master -> PTY -> test slave peer.
	commands <- WriteCoilCommand(1, 3, true)
	// Nest master -> event channel -> test after peer write completion.
	event := receiveEvent(t, events)
	assert.Equal(t, WriteSucceededEventKind, event.Kind)
	assert.True(t, coils[3])

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for master shutdown")
	}

	// Original slave handle close; peer master read unblocked after actor shutdown.
	assert.NoError(t, slave.Close())
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for PTY peer shutdown")
	}
}
