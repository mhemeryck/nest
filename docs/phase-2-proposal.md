# Phase 2 Proposal

## Status

This proposal is now mostly implemented.
It is kept as a record of the Phase 2 boundary, but the code has moved slightly beyond the original scope.

The main difference is that local `lights` and `bindings` are now included.
That lets `nest` execute a local `push_button -> light -> relay` path rather than stopping at inventory and event translation.

## Goal

Phase 2 should introduce the smallest config model that lets `nest` describe one local unit.
That model should be enough to define local digital inputs, push buttons, relays, lights, and local bindings without pulling in MQTT, Modbus, or cover control yet.

This keeps the next phase focused on local input semantics rather than config redesign.

## Scope

Phase 2 should deliver these concrete things.

1. A YAML file format for one unit.
2. A loader that reads and validates that file.
3. Internal types that represent local digital inputs, push buttons, relays, lights, and bindings.
4. A CLI entrypoint that accepts a config path instead of hardcoding fixture paths.
5. A local execution path from push button events to light actions.

Phase 2 should not include transport config, Home Assistant integration, distributed mappings, motors, or covers.

## Proposed Config Shape

This proposal keeps the file local-only.
It makes hardware devices, semantic input devices, local light entities, and local bindings explicit.

```yaml
sysfs:
  root: test/fixtures

digital_inputs:
  - id: office_button_input
    device: di_3_16

push_buttons:
  - id: office_button
    name: Office light button
    input: office_button_input

relays:
  - id: office_light_relay
    name: Office light relay
    device: ro_3_14

lights:
  - id: office_light
    name: Office light
    relay: office_light_relay

bindings:
  - button: office_button
    light: office_light
    action: toggle
```

## Why This Shape

This shape keeps hardware details at the edges.
`digital_inputs` and `relays` map directly to sysfs device identifiers.
`push_buttons` are semantic devices layered on top of raw digital inputs.
`lights` are semantic actuators layered on top of relays.

That gives us a clear separation.

1. Hardware inventory: `digital_inputs` and `relays`.
2. Semantic local input devices: `push_buttons`.
3. Semantic local output devices: `lights`.
4. Local behavior wiring: `bindings`.

This is enough for the next local input slice.
It also leaves room to add `motors`, `covers`, `mqtt`, and `serial` later without reshaping the low-level model.

## YAML Rules

The loader should enforce these rules.

1. `digital_inputs[*].id`, `push_buttons[*].id`, `lights[*].id`, and `relays[*].id` must be unique within their section.
2. `device` values must use the existing sysfs naming style such as `di_2_15` and `ro_3_14`.
3. `digital_inputs[*].device` must match a digital input identifier.
4. `relays[*].device` must match a relay identifier.
5. `push_buttons[*].input` must reference an existing digital input id.
6. `lights[*].relay` must reference an existing relay id.
7. `bindings[*].button` must reference an existing push button id.
8. `bindings[*].light` must reference an existing light id.
9. `bindings[*].action` must use a supported action.

The loader should fail fast with explicit field-level errors.
Unknown YAML fields should also fail validation so configuration drift is caught early.

## Proposed Go Types

These types live in `internal/config`.

```go
package config

type Root struct {
	Sysfs         SysfsConfig          `yaml:"sysfs"`
	DigitalInputs []DigitalInputConfig `yaml:"digital_inputs"`
	PushButtons   []PushButtonConfig   `yaml:"push_buttons"`
	Lights        []LightConfig        `yaml:"lights"`
	Relays        []RelayConfig        `yaml:"relays"`
	Bindings      []BindingConfig      `yaml:"bindings"`
}

type SysfsConfig struct {
	Root string `yaml:"root"`
}

type DigitalInputConfig struct {
	ID     string `yaml:"id"`
	Device string `yaml:"device"`
}

type PushButtonConfig struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Input string `yaml:"input"`
}

type RelayConfig struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Device string `yaml:"device"`
}

type LightConfig struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name"`
	Relay string `yaml:"relay"`
}

type BindingConfig struct {
	Button string `yaml:"button"`
	Light  string `yaml:"light"`
	Action string `yaml:"action"`
}
```

The first loader API can stay small.

```go
func Load(path string) (*Root, error)
func Validate(f *Root) error
```

## Runtime Model

Phase 2 should keep runtime data procedural and explicit.
That means startup-built lookup maps rather than an object graph of linked structs.

These do not need to know anything about YAML tags.

```go
type Runtime struct {
	DigitalInputsByDevice map[string]config.DigitalInputConfig
	ButtonsByInputID      map[string][]config.PushButtonConfig
	LightsByID            map[string]config.LightConfig
	BindingsByButtonID    map[string][]config.BindingConfig
}
```

The important part is not the exact package name.
The important part is separating parsed config from runtime indexes.
The program should build those indexes at startup and keep them for the entire process lifetime.
In the current code this lives in `internal/registry` rather than `internal/runtime`.

## Event Pipeline

The next layer after config should be a shared in-process event bus.
The bus should carry a single top-level event type with one payload per event kind.
That gives `nest` one event stream while keeping event payloads explicit.

```go
type EventKind string

const (
	EventDigitalInput EventKind = "digital_input"
	EventPushButton   EventKind = "push_button"
)

type Event struct {
	Kind         EventKind
	DigitalInput *DigitalInputEvent
	PushButton   *PushButtonEvent
}

type DigitalInputEvent struct {
	InputID   string
	DeviceID  string
	IsRising  bool
	IsFalling bool
}

type PushButtonEvent struct {
	ButtonID string
	Name     string
	Kind     string
}
```

This is a tagged-union style event envelope.
It fits Go well and keeps the dispatch logic explicit.

The expected flow is:

1. `sysfs` polling emits `DigitalInputEvent` values.
2. An input mapper resolves those to configured `push_buttons`.
3. The mapper emits `PushButtonEvent` values into the controller dispatch flow.
4. The controller resolves configured bindings for that button.
5. The binding resolves to a light action.
6. The light resolves to its relay target.

For the first cut, `PushButtonEvent.Kind` only needs `pressed`.
Later phases can add `released`, `long_press`, and similar higher-level semantics.

## CLI Proposal

The hardcoded fixture root in `cmd/nest/main.go` should be replaced by a config path.

The minimal contract should be:

```text
nest run --config /etc/nest/config.yaml
nest validate --config ./config.yaml
```

`run` should load config, validate it, and start the controller.
`validate` should load config, validate it, and exit without touching hardware.

If we want to keep the first CLI even smaller, we can skip subcommands and support only:

```text
nest --config ./config.yaml
```

That is acceptable for Phase 2.
Adding `validate` early is still useful because it gives a clean way to test config changes without running the controller.

## Package Boundaries

This is the smallest package split that proved worth introducing.

1. `internal/config`
Reads YAML and validates config.
2. `internal/entity`
Defines the typed local domain model.
3. `internal/sysfs`
Continues to own device discovery and low-level reads and writes.
4. `internal/event`
Defines shared event types and the in-process event bus.
5. `internal/registry`
Builds startup lookup maps from entities.
6. `internal/controller`
Owns local event dispatch and automation execution.
7. `cmd/nest`
Owns CLI parsing and process wiring.

Phase 2 turned out to justify a dedicated controller package once local bindings were added.

## Validation Details

The loader should use `yaml.Decoder.KnownFields(true)` so misspelled fields are rejected.

Validation errors should include enough context to fix the file quickly.
Examples:

1. `push_buttons[0].input: unknown digital input "office_button_input"`
2. `digital_inputs[0].device: invalid digital input device "ro_3_14"`
3. `relays[1].device: invalid relay device "di_2_03"`
4. `lights[0].relay: unknown relay "office_light_relay"`
5. `bindings[0].light: unknown light "office_light"`

This will matter once the config gets larger and moves out of the repository.

## Suggested Delivery Order

Phase 2 was delivered in roughly this order.

1. Add `internal/config` with file structs and `Load`.
2. Add validation and table-driven tests.
3. Add startup-built runtime indexes.
4. Replace the hardcoded path in `cmd/nest/main.go` with a config-driven path.
5. Add a sample local config under `test/fixtures` or `docs`.
6. Add a shared event bus and map `DigitalInputEvent` to `PushButtonEvent`.
7. Add local light and binding types.
8. Resolve push button events to light actions and relay toggles.

## Explicit Non-Goals

These should wait until later phases.

1. MQTT settings.
2. Serial or Modbus settings.
3. Multi-unit light mappings.
4. Motors and covers.
5. Position tracking or restart recovery.

## Recommendation

Build Phase 2 around a local-only YAML file with `digital_inputs`, `push_buttons`, `relays`, `lights`, and `bindings`.
Keep transport and covers out of the file for now.
Add one config loader, one validator, startup lookup maps, a shared tagged-union event flow, and a small local automation path.

That gives the next phase a stable base for implementing richer input semantics without dragging in unrelated cover or transport design decisions.
