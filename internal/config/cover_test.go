package config

import (
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/mhemeryck/nest/internal/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const coverFixture = `
units:
  shady:
    actors:
      sysfs:
        root: /tmp/sysfs
        digital_inputs:
          - id: up
            device: di_1_01
          - id: down
            device: di_1_02
        relays:
          - id: up
            name: Up
            device: ro_1_01
          - id: down
            name: Down
            device: ro_1_02
    entities:
      buttons:
        - id: up
          name: Up
          input:
            actor: sysfs
            kind: digital_input
            id: up
        - id: down
          name: Down
          input:
            actor: sysfs
            kind: digital_input
            id: down
      covers:
        - id: office
          name: Office
          open_actuator:
            actor: sysfs
            kind: relay
            id: up
          close_actuator:
            actor: sysfs
            kind: relay
            id: down
          movement_timeout: 30s
          reversal_delay: 500ms
bindings:
  - source: shady.button.up
    target: shady.cover.office
    action: hold_open
  - source: shady.button.down
    target: shady.cover.office
    action: hold_close
`

func TestCoverProjectionAndRegistry(t *testing.T) {
	global, err := decodeGlobalRoot("cover.yaml", []byte(coverFixture))
	require.NoError(t, err)
	local, err := ProjectUnit(global, "shady")
	require.NoError(t, err)
	require.NoError(t, Validate(local))
	reg := registry.Build(ToEntityRoot(local))
	cover, ok := registry.CoverByID(reg, "shady.cover.office")
	require.True(t, ok)
	assert.Equal(t, entity.Cover{
		ID: "shady.cover.office", Name: "Office", OpenRelay: "up", CloseRelay: "down",
		MovementTimeout: 30 * time.Second, ReversalDelay: 500 * time.Millisecond,
	}, cover)
	for _, relayID := range []entity.RelayID{cover.OpenRelay, cover.CloseRelay} {
		owner, found := registry.CoverByRelay(reg, relayID)
		require.True(t, found)
		assert.Equal(t, cover, owner)
		relay, found := registry.RelayByID(reg, relayID)
		require.True(t, found)
		assert.NotEmpty(t, relay.SysfsDevice)
	}
	for _, test := range []struct {
		source entity.PushButtonID
		action entity.Action
	}{
		{"shady.button.up", entity.ActionHoldOpen}, {"shady.button.down", entity.ActionHoldClose},
	} {
		bindings := registry.BindingsByButton(reg, test.source)
		require.Len(t, bindings, 1)
		assert.Equal(t, entity.ID(test.source), bindings[0].Source)
		assert.Equal(t, entity.ID(cover.ID), bindings[0].Target)
		assert.Equal(t, test.action, bindings[0].Action)
	}
	covers := registry.Covers(reg)
	covers[0].Name = "Changed"
	assert.Equal(t, cover, registry.Covers(reg)[0])
	_, ok = registry.CoverByID(reg, "missing")
	assert.False(t, ok)
}

func TestInvalidCoverConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Root)
		message string
	}{
		{"missing relay", func(r *Root) { r.Covers[0].OpenRelay = "missing" }, "unknown relay"},
		{"same relay", func(r *Root) { r.Covers[0].CloseRelay = "up" }, "already owned"},
		{"light ownership", func(r *Root) { r.Lights = []LightConfig{{ID: "shady.light.office", Name: "Office", Relay: "up"}} }, "already owned"},
		{"cover ownership", func(r *Root) { c := r.Covers[0]; c.ID = "shady.cover.other"; r.Covers = append(r.Covers, c) }, "already owned"},
		{"duplicate id", func(r *Root) { r.Covers = append(r.Covers, r.Covers[0]) }, "duplicate"},
		{"invalid id", func(r *Root) { r.Covers[0].ID = "shady.cover.bad-id" }, "covers[0].id"},
		{"missing name", func(r *Root) { r.Covers[0].Name = "" }, "covers[0].name"},
		{"missing timeout", func(r *Root) { r.Covers[0].MovementTimeout = 0 }, "movement_timeout: must be positive"},
		{"negative timeout", func(r *Root) { r.Covers[0].MovementTimeout = -time.Second }, "movement_timeout: must be positive"},
		{"missing delay", func(r *Root) { r.Covers[0].ReversalDelay = 0 }, "reversal_delay: must be positive"},
		{"negative delay", func(r *Root) { r.Covers[0].ReversalDelay = -time.Second }, "reversal_delay: must be positive"},
		{"unknown cover", func(r *Root) { r.Bindings[0].Target = "shady.cover.missing" }, "unknown cover"},
		{"unknown button", func(r *Root) { r.Bindings[0].Source = "shady.button.missing" }, "unknown push button"},
		{"wrong action", func(r *Root) { r.Bindings[0].Action = "toggle" }, "expected hold_open or hold_close"},
		{"conflicting direction", func(r *Root) { b := r.Bindings[0]; b.Action = "hold_close"; r.Bindings = append(r.Bindings, b) }, "conflicting cover binding"},
		{"duplicate binding", func(r *Root) { r.Bindings = append(r.Bindings, r.Bindings[0]) }, "duplicate"},
		{"transport", func(r *Root) { r.Bindings[0].ExecutionTransport = "mqtt" }, "must be local"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			global, err := decodeGlobalRoot("cover.yaml", []byte(coverFixture))
			require.NoError(t, err)
			root, err := ProjectUnit(global, "shady")
			require.NoError(t, err)
			test.mutate(root)
			require.ErrorContains(t, Validate(root), test.message)
		})
	}
}

func TestInvalidGlobalCoverConfiguration(t *testing.T) {
	for _, test := range []struct {
		name    string
		mutate  func(*GlobalRoot)
		message string
	}{
		{"endpoint actor", func(g *GlobalRoot) { g.Units["shady"].Entities.Covers[0].OpenActuator.Actor = "mqtt" }, "unsupported endpoint actor"},
		{"endpoint kind", func(g *GlobalRoot) { g.Units["shady"].Entities.Covers[0].CloseActuator.Kind = "digital_input" }, "unsupported endpoint kind"},
		{"unknown target", func(g *GlobalRoot) { g.Bindings[0].Target = "shady.cover.missing" }, "unknown cover"},
		{"cross unit", func(g *GlobalRoot) { g.Units["other"] = g.Units["shady"]; g.Bindings[0].Source = "other.button.up" }, "same unit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			global, err := decodeGlobalRoot("cover.yaml", []byte(coverFixture))
			require.NoError(t, err)
			test.mutate(global)
			_, err = ProjectUnit(global, "shady")
			require.ErrorContains(t, err, test.message)
		})
	}
}
