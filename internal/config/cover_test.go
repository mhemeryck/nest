package config

import (
	"testing"
	"time"

	"github.com/mhemeryck/nest/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoverParsingAndProjection(t *testing.T) {
	global, err := decodeGlobalRoot("covers.yaml", []byte(`
cover_control:
  full_travel_duration: 30s
  operation_timeout: 2s
  off_retry_interval: 1s
  shutdown_period: 5s
  reporting_interval: 500ms
units:
  controller_1:
    actors:
      persistence: {path: /var/lib/nest/controller_1.json}
      sysfs:
        root: /sys/devices
        relays:
          - {id: office_open, name: Office open, device: ro_1_1}
          - {id: office_close, name: Office close, device: ro_1_2}
    entities:
      covers:
        - id: office
          name: Office cover
          open: {actor: sysfs, kind: relay, id: office_open}
          close: {actor: sysfs, kind: relay, id: office_close}
`))
	require.NoError(t, err)
	local, err := ProjectUnit(global, "controller_1")
	require.NoError(t, err)
	require.NoError(t, Validate(local))
	assert.Equal(t, []CoverConfig{{ID: "controller_1.cover.office", Name: "Office cover", OpenRelay: "office_open", CloseRelay: "office_close"}}, local.Covers)
	assert.Equal(t, entity.CoverControl{
		FullTravelDuration: 30 * time.Second, OperationTimeout: 2 * time.Second,
		OffRetryInterval: time.Second, ShutdownPeriod: 5 * time.Second, ReportingInterval: 500 * time.Millisecond,
	}, ToEntityRoot(local).CoverControl)
	assert.Equal(t, global.CoverControl, local.CoverControl)
	assert.Equal(t, "/var/lib/nest/controller_1.json", ToEntityRoot(local).PersistencePath)
}

func TestCoverControlPositiveDurations(t *testing.T) {
	settings := CoverControlConfig{
		FullTravelDuration: time.Second, OperationTimeout: time.Second,
		OffRetryInterval: time.Second, ShutdownPeriod: time.Second, ReportingInterval: time.Second,
	}
	require.NoError(t, validateCoverControl(settings, true))
	for _, field := range []string{"full_travel_duration", "operation_timeout", "off_retry_interval", "shutdown_period", "reporting_interval"} {
		for _, duration := range []time.Duration{0, -time.Second} {
			t.Run(field+"/"+duration.String(), func(t *testing.T) {
				invalid := settings
				switch field {
				case "full_travel_duration":
					invalid.FullTravelDuration = duration
				case "operation_timeout":
					invalid.OperationTimeout = duration
				case "off_retry_interval":
					invalid.OffRetryInterval = duration
				case "shutdown_period":
					invalid.ShutdownPeriod = duration
				case "reporting_interval":
					invalid.ReportingInterval = duration
				}
				assert.ErrorContains(t, validateCoverControl(invalid, true), "cover_control."+field)
			})
		}
	}
	assert.NoError(t, validateCoverControl(CoverControlConfig{}, false))
}

func TestCoverProjectionRejectsInvalidEndpoints(t *testing.T) {
	for _, endpoint := range []EndpointRefConfig{
		{Actor: "mqtt", Kind: "relay", ID: "open"},
		{Actor: "sysfs", Kind: "digital_input", ID: "open"},
		{Actor: "sysfs", Kind: "relay", ID: "invalid-id"},
	} {
		_, err := projectCovers("controller_1", []UnitCoverConfig{{
			ID: "office", Open: endpoint, Close: EndpointRefConfig{Actor: "sysfs", Kind: "relay", ID: "close"},
		}})
		assert.Error(t, err)
	}
}

func TestCoverRelayOwnership(t *testing.T) {
	cover := CoverConfig{ID: "office", Name: "Office", OpenRelay: "open", CloseRelay: "close"}
	relays := map[string]struct{}{"open": {}, "close": {}, "other": {}}
	_, err := validateCovers([]CoverConfig{cover}, nil, relays)
	require.NoError(t, err)
	for _, tc := range []struct {
		name    string
		covers  []CoverConfig
		lights  []LightConfig
		message string
	}{
		{name: "same relay", covers: []CoverConfig{{ID: "office", Name: "Office", OpenRelay: "open", CloseRelay: "open"}}, message: "already owned"},
		{name: "another cover", covers: []CoverConfig{cover, {ID: "hall", Name: "Hall", OpenRelay: "close", CloseRelay: "other"}}, message: "already owned"},
		{name: "light", covers: []CoverConfig{cover}, lights: []LightConfig{{ID: "light", Relay: "open"}}, message: "owned by light"},
		{name: "unknown relay", covers: []CoverConfig{{ID: "office", Name: "Office", OpenRelay: "missing", CloseRelay: "close"}}, message: "unknown relay"},
		{name: "duplicate id", covers: []CoverConfig{cover, cover}, message: "duplicate cover id"},
		{name: "wrong entity type", covers: []CoverConfig{{ID: "unit.light.office", Name: "Office", OpenRelay: "open", CloseRelay: "close"}}, message: "cover semantic id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateCovers(tc.covers, tc.lights, relays)
			assert.ErrorContains(t, err, tc.message)
		})
	}
}

func TestGlobalCoverBindingsProjectToBothUnits(t *testing.T) {
	global := &GlobalRoot{Units: map[string]UnitConfig{
		"source": {Entities: UnitEntitiesConfig{Buttons: []UnitPushButtonConfig{{ID: "button", Input: EndpointRefConfig{Actor: "sysfs", Kind: "digital_input", ID: "input"}}}}},
		"target": {Entities: UnitEntitiesConfig{Covers: []UnitCoverConfig{{ID: "cover", Open: EndpointRefConfig{Actor: "sysfs", Kind: "relay", ID: "open"}, Close: EndpointRefConfig{Actor: "sysfs", Kind: "relay", ID: "close"}}}}},
	}, Bindings: []GlobalBindingConfig{{Source: "source.button.button", Target: "target.cover.cover", Action: "open", ExecutionTransport: "mqtt"}}}
	source, err := ProjectUnit(global, "source")
	require.NoError(t, err)
	require.Len(t, source.RemoteSourceBindings, 1)
	target, err := ProjectUnit(global, "target")
	require.NoError(t, err)
	require.Len(t, target.RemoteTargetBindings, 1)
	assert.Equal(t, source.RemoteSourceBindings, target.RemoteTargetBindings)
	global.Bindings[0].ExecutionTransport = "modbus"
	_, err = ProjectUnit(global, "target")
	assert.ErrorContains(t, err, "Modbus cover bindings are not supported")
	global.Bindings[0].ExecutionTransport = "mqtt"
	global.Bindings[0].Action = "toggle"
	_, err = ProjectUnit(global, "target")
	assert.ErrorContains(t, err, "unsupported cover action")
}

func TestLocalCoverBindingValidation(t *testing.T) {
	buttons := map[string]struct{}{"button": {}}
	covers := map[string]struct{}{"cover": {}}
	for _, action := range []string{"open", "close", "stop"} {
		assert.NoError(t, validateBindings([]BindingConfig{{Source: "button", Target: "cover", Action: action}}, buttons, nil, covers))
	}
	assert.ErrorContains(t, validateBindings([]BindingConfig{{Source: "button", Target: "cover", Action: "toggle"}}, buttons, nil, covers), "unsupported cover action")
}
