package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCovers(t *testing.T) {
	root, err := Load(filepath.Join("..", "..", "test", "fixtures", "config.covers.yaml"), "controller_1")
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, root.CoverControl.MaxRun)
	assert.Equal(t, []CoverConfig{{
		ID: "controller_1.cover.office", Name: "Office", OpenRelay: "office_up", CloseRelay: "office_down",
	}}, root.Covers)
	require.Len(t, root.Lights, 1)
	assert.Empty(t, root.Bindings)
	assert.False(t, root.MQTT.Enabled)

	entities := ToEntityRoot(root)
	assert.Equal(t, entity.CoverControl{MaxRun: 30 * time.Second}, entities.CoverControl)
	assert.Equal(t, []entity.Cover{{
		ID: entity.CoverID("controller_1.cover.office"), Name: "Office",
		OpenRelay: entity.RelayID("office_up"), CloseRelay: entity.RelayID("office_down"),
	}}, entities.Covers)
}

func TestProjectSharedCoverControl(t *testing.T) {
	for _, maxRun := range []time.Duration{0, 45 * time.Second} {
		global := coverFixture(t)
		global.CoverControl.MaxRun = maxRun
		expected := maxRun
		if expected == 0 {
			expected = 30 * time.Second
		}
		for _, unitID := range []string{"controller_1", "controller_2"} {
			root, err := ProjectUnit(global, unitID)
			require.NoError(t, err)
			require.NoError(t, Validate(root))
			assert.Equal(t, expected, root.CoverControl.MaxRun)
			require.Len(t, root.Covers, 1)
			assert.True(t, entity.IsIDForUnit(root.Covers[0].ID, unitID, entity.TypeCover))
			assert.Equal(t, maxRun, global.CoverControl.MaxRun, "projection must not mutate global settings")
		}
	}
}

func TestValidateCovers(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Root)
		error  string
	}{
		{"missing id", func(root *Root) { root.Covers[0].ID = "" }, "covers[0].id: required"},
		{"invalid id", func(root *Root) { root.Covers[0].ID = "Office" }, "covers[0].id: must be"},
		{"wrong semantic type", func(root *Root) { root.Covers[0].ID = "controller_1.light.office" }, "covers[0].id: must be"},
		{"missing name", func(root *Root) { root.Covers[0].Name = "" }, "covers[0].name: required"},
		{"duplicate id", func(root *Root) { root.Covers = append(root.Covers, root.Covers[0]) }, "duplicate id"},
		{"missing open relay", func(root *Root) { root.Covers[0].OpenRelay = "" }, "covers[0].open_relay: required"},
		{"missing close relay", func(root *Root) { root.Covers[0].CloseRelay = "" }, "covers[0].close_relay: required"},
		{"unknown open relay", func(root *Root) { root.Covers[0].OpenRelay = "missing" }, "covers[0].open_relay: unknown relay"},
		{"unknown close relay", func(root *Root) { root.Covers[0].CloseRelay = "missing" }, "covers[0].close_relay: unknown relay"},
		{"same direction relay", func(root *Root) { root.Covers[0].CloseRelay = root.Covers[0].OpenRelay }, "already used by covers[0].open_relay"},
		{"shared with light", func(root *Root) { root.Lights[0].Relay = root.Covers[0].OpenRelay }, "already used by lights[0].relay"},
		{"shared between covers", func(root *Root) {
			root.Covers = append(root.Covers, CoverConfig{ID: "other", Name: "Other", OpenRelay: "office_up", CloseRelay: "office_down"})
		}, "covers[1].open_relay: relay device"},
		{"physical alias", func(root *Root) {
			root.Relays = append(root.Relays, RelayConfig{ID: "alias", Name: "Alias", Device: "ro_3_14"})
			root.Lights[0].Relay = "alias"
		}, "already used by lights[0].relay"},
		{"zero duration", func(root *Root) { root.CoverControl.MaxRun = 0 }, "cover_control.max_run: must be positive"},
		{"negative duration", func(root *Root) { root.CoverControl.MaxRun = -time.Second }, "cover_control.max_run: must be positive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := ProjectUnit(coverFixture(t), "controller_1")
			require.NoError(t, err)
			tt.mutate(root)
			err = Validate(root)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.error)
		})
	}
}

func TestProjectRejectsInvalidCoverEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint EndpointRefConfig
	}{
		{"actor", EndpointRefConfig{Actor: "modbus", Kind: "relay", ID: "office_up"}},
		{"kind", EndpointRefConfig{Actor: "sysfs", Kind: "digital_input", ID: "office_up"}},
		{"id", EndpointRefConfig{Actor: "sysfs", Kind: "relay", ID: ""}},
	}
	for _, direction := range []string{"open_actuator", "close_actuator"} {
		for _, tt := range tests {
			t.Run(direction+"/"+tt.name, func(t *testing.T) {
				global := coverFixture(t)
				unit := global.Units["controller_1"]
				if direction == "open_actuator" {
					unit.Entities.Covers[0].OpenActuator = tt.endpoint
				} else {
					unit.Entities.Covers[0].CloseActuator = tt.endpoint
				}
				_, err := ProjectUnit(global, "controller_1")
				require.Error(t, err)
				assert.Contains(t, err.Error(), "entities.covers[0]."+direction+"."+tt.name)
			})
		}
	}
}

func TestDecodeRejectsPerCoverRunDuration(t *testing.T) {
	_, err := decodeGlobalRoot("test", []byte(`
units:
  controller_1:
    entities:
      covers:
        - id: office
          max_run: 30s
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field max_run not found")
}

func coverFixture(t *testing.T) *GlobalRoot {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "test", "fixtures", "config.covers.yaml"))
	require.NoError(t, err)
	global, err := decodeGlobalRoot("config.covers.yaml", data)
	require.NoError(t, err)
	return global
}
