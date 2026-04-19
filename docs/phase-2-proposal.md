# Phase 2 Proposal

## Goal

Phase 2 should introduce the smallest config model that lets `nest` describe one local unit.
That model should be enough to wire local inputs, relays, and a first cover entity without pulling in MQTT or Modbus concerns.

This keeps the next phase focused on cover behavior rather than config redesign.

## Scope

Phase 2 should deliver four concrete things.

1. A YAML file format for one unit.
2. A loader that reads and validates that file.
3. Internal types that represent local inputs, relays, and covers.
4. A CLI entrypoint that accepts a config path instead of hardcoding fixture paths.

Phase 2 should not include transport config, Home Assistant integration, or distributed mappings.

## Proposed Config Shape

This proposal keeps the file local-only.
It borrows the cover example from `docs/design.md`, but makes identifiers explicit so later phases can reference entities without depending on display names.

```yaml
sysfs:
  root: /

inputs:
  - id: office-shade-button
    name: Office shade toggle
    device: di_2_15

relays:
  - id: office-shade-up
    name: Office shade up
    device: ro_3_14
  - id: office-shade-down
    name: Office shade down
    device: ro_3_13

covers:
  - id: office
    name: Office shade
    up_relay: office-shade-up
    down_relay: office-shade-down
    max_time: 30s

bindings:
  - input: office-shade-button
    target: office
    action: toggle
```

## Why This Shape

This shape keeps hardware details at the edges.
Inputs and relays map directly to sysfs device identifiers.
Entities such as covers reference relay ids rather than raw sysfs names.

That gives us a clear separation.

1. Hardware inventory: `inputs` and `relays`.
2. Local domain entities: `covers`.
3. Behavior wiring: `bindings`.

This is enough for the single-unit shade slice described in the implementation plan.
It also leaves room to add `lights`, `mqtt`, and `serial` later without reshaping the local model.

## YAML Rules

The loader should enforce these rules.

1. `inputs[*].id`, `relays[*].id`, and `covers[*].id` must be unique within their section.
2. `device` values must use the existing sysfs naming style such as `di_2_15` and `ro_3_14`.
3. `covers[*].up_relay` and `covers[*].down_relay` must reference existing relay ids.
4. `covers[*].up_relay` and `covers[*].down_relay` must not be the same relay.
5. `bindings[*].input` must reference an existing input id.
6. `bindings[*].target` must reference an existing entity id.
7. `bindings[*].action` should initially allow only `toggle`, `open`, `close`, and `stop`.
8. `max_time` must be a positive duration.

The loader should fail fast with explicit field-level errors.
Unknown YAML fields should also fail validation so configuration drift is caught early.

## Proposed Go Types

These types should live in a new `internal/config` package.

```go
package config

import "time"

type File struct {
	Sysfs    SysfsConfig     `yaml:"sysfs"`
	Inputs   []InputConfig   `yaml:"inputs"`
	Relays   []RelayConfig   `yaml:"relays"`
	Covers   []CoverConfig   `yaml:"covers"`
	Bindings []BindingConfig `yaml:"bindings"`
}

type SysfsConfig struct {
	Root string `yaml:"root"`
}

type InputConfig struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Device string `yaml:"device"`
}

type RelayConfig struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Device string `yaml:"device"`
}

type CoverConfig struct {
	ID        string        `yaml:"id"`
	Name      string        `yaml:"name"`
	UpRelay   string        `yaml:"up_relay"`
	DownRelay string        `yaml:"down_relay"`
	MaxTime   time.Duration `yaml:"max_time"`
}

type BindingConfig struct {
	Input  string `yaml:"input"`
	Target string `yaml:"target"`
	Action string `yaml:"action"`
}
```

The first loader API can stay small.

```go
func Load(path string) (*File, error)
func (f *File) Validate() error
```

## Runtime Model

Phase 2 should also introduce simple runtime structs for resolved local entities.
These do not need to know anything about YAML tags.

```go
package domain

import "time"

type Input struct {
	ID     string
	Name   string
	Device string
}

type Relay struct {
	ID     string
	Name   string
	Device string
}

type Cover struct {
	ID        string
	Name      string
	UpRelay   Relay
	DownRelay Relay
	MaxTime   time.Duration
}

type Binding struct {
	InputID  string
	TargetID string
	Action   string
}
```

The important part is not the exact package name.
The important part is separating parsed config from resolved runtime objects.

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

This is the smallest package split that seems worth introducing now.

1. `internal/config`
Reads YAML and validates config.
2. `internal/domain`
Holds resolved local entities and bindings.
3. `internal/sysfs`
Continues to own device discovery and low-level reads and writes.
4. `cmd/nest`
Owns CLI parsing and process wiring.

Phase 2 does not need a dedicated controller package yet unless the first cover control code clearly needs it.

## Validation Details

The loader should use `yaml.Decoder.KnownFields(true)` so misspelled fields are rejected.

Validation errors should include enough context to fix the file quickly.
Examples:

1. `covers[0].up_relay: unknown relay id "office-up"`
2. `bindings[0].action: unsupported action "press"`
3. `relays[1].device: invalid relay device "di_2_03"`

This will matter once the config gets larger and moves out of the repository.

## Suggested Delivery Order

Implement Phase 2 in this order.

1. Add `internal/config` with file structs and `Load`.
2. Add validation and table-driven tests.
3. Add resolved runtime structs and a small conversion step from config to runtime.
4. Replace the hardcoded path in `cmd/nest/main.go` with a config-driven path.
5. Add a sample local cover config under `test/fixtures` or `docs`.

## Explicit Non-Goals

These should wait until later phases.

1. MQTT settings.
2. Serial or Modbus settings.
3. Multi-unit light mappings.
4. Debounce or press semantics beyond a simple action string.
5. Position tracking or restart recovery.

## Recommendation

Build Phase 2 around a local-only YAML file with `inputs`, `relays`, `covers`, and `bindings`.
Keep transport out of the file for now.
Add one config loader, one validator, and one config-driven CLI path.

That gives Phase 3 a stable base for implementing single-unit cover control without dragging in unrelated design decisions.
