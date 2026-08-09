# Long-Term Ideas

Deferred ideas.

## Input Model

- Button `released`, `long_press`, repeat, and duration semantics
- Multiple buttons per digital input
- Debounce in the input event pipeline
- Duration-driven dimmer control

## Automation Model

- Button bindings for relays, motors, and covers
- Semantic-event automation handler
- Explicit action-to-command model
- Short- and long-press light actions

## Actuator Model

- Two-relay motor actuators
- Covers built on motors
- Separate motor and cover state

## Event System

- Actuator and automation event types
- Event tracing and structured flow logging
- Controller-centered integration topology

## Transport And Integration

- GPIO v2 ABI over UniPi sysfs
- Home Assistant discovery contract refinements
- Existing MQTT-facing options in discovery; no separate HA config layer
- Configuration-management API
- Terraform provider for that API

## Operations

- Dedicated `validate` command
- Example configs: input-only, relay-only, local cover
- Startup validation against live sysfs state
- Reproducible NixOS UniPi images
- Simple server-driven local web UI, likely `htmx`
