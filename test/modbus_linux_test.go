//go:build linux

package integration_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
)

func TestModbusButtonTogglesRemoteRelay(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "nest")
	output, err := exec.CommandContext(t.Context(), "go", "build", "-o", binary, "../cmd/nest").CombinedOutput()
	require.NoError(t, err, "%s", output)

	teslaPort, teslaWire := serialEndpoint(t)
	edisonPort, edisonWire := serialEndpoint(t)
	// Bridge the PTY masters so each Nest process can open its own serial path.
	go func() { _, _ = io.Copy(teslaWire, edisonWire) }()
	go func() { _, _ = io.Copy(edisonWire, teslaWire) }()

	teslaRoot := filepath.Join(dir, "tesla")
	edisonRoot := filepath.Join(dir, "edison")
	button := writeMockValue(t, teslaRoot, "di_3_16/di_value")
	relay := writeMockValue(t, edisonRoot, "ro_3_12/ro_value")
	config := filepath.Join(dir, "nest.yaml")
	require.NoError(t, os.WriteFile(config, fmt.Appendf(nil, pilotConfig,
		teslaPort, teslaRoot, edisonPort, edisonRoot), 0o600))

	startNest(t, binary, config, "edison")
	teslaLog := startNest(t, binary, config, "tesla")
	require.Equal(t, "0", readText(t, relay))

	require.NoError(t, os.WriteFile(button, []byte("1\n"), 0o600))
	waitForValue(t, relay, "1")
	require.NoError(t, os.WriteFile(button, []byte("0\n"), 0o600))
	waitForLog(t, teslaLog, "push_button_released")
	require.Equal(t, "1", readText(t, relay))
	require.NoError(t, os.WriteFile(button, []byte("1\n"), 0o600))
	waitForValue(t, relay, "0")
}

func serialEndpoint(t *testing.T) (string, *os.File) {
	t.Helper()
	wire, port, err := pty.Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = port.Close()
		_ = wire.Close()
	})
	return port.Name(), wire
}

func writeMockValue(t *testing.T, root, device string) string {
	t.Helper()
	path := filepath.Join(root, "sys/devices/platform/unipi_plc/io_group3", device)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("0\n"), 0o600))
	return path
}

func startNest(t *testing.T, binary, config, unit string) string {
	t.Helper()
	log, err := os.Create(filepath.Join(filepath.Dir(config), unit+".log"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	cmd := exec.CommandContext(t.Context(), binary, "--config", config, unit)
	cmd.Stdout, cmd.Stderr = log, log
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("%s log:\n%s", unit, readText(t, log.Name()))
		}
	})
	waitForLog(t, log.Name(), "runtime started")
	return log.Name()
}

func waitForLog(t *testing.T, path, message string) {
	t.Helper()
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(path)
		return err == nil && strings.Contains(string(data), message)
	}, 5*time.Second, 10*time.Millisecond, "expected %q in %s", message, path)
}

func waitForValue(t *testing.T, path, value string) {
	t.Helper()
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(path)
		return err == nil && strings.TrimSpace(string(data)) == value
	}, 5*time.Second, 10*time.Millisecond, "expected %q in %s", value, path)
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSpace(string(data))
}

const pilotConfig = `
units:
  tesla:
    actors:
      modbus:
        mode: master
        port: %q
        baudrate: 19200
        timeout: 500ms
        event_signal_writes:
          - unit: edison
            signal: button_toggle
            source: tesla.button.wall
            target: edison.light.lamp
            action: toggle
      sysfs:
        root: %q
        digital_inputs:
          - id: input
            device: di_3_16
    entities:
      buttons:
        - id: wall
          name: Wall button
          input:
            actor: sysfs
            kind: digital_input
            id: input
  edison:
    actors:
      modbus:
        mode: slave
        port: %q
        baudrate: 19200
        timeout: 500ms
        unit_id: 1
        event_signals:
          - id: button_toggle
            coil: 1
            source: tesla.button.wall
            target: edison.light.lamp
            action: toggle
      sysfs:
        root: %q
        relays:
          - id: relay
            name: Lamp relay
            device: ro_3_12
    entities:
      lights:
        - id: lamp
          name: Lamp
          actuator:
            actor: sysfs
            kind: relay
            id: relay
bindings:
  - source: tesla.button.wall
    target: edison.light.lamp
    action: toggle
    execution_transport: modbus
`
