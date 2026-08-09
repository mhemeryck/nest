# Long-Term Ideas

This document is for ideas that seem useful but are not part of the
current implementation slice.
Keep items here short so they are easy to scan and revisit later.

## Input Model

- Add richer push button semantics such as `released`, `long_press`, and repeated presses.
- Support multiple push buttons bound to the same digital input when that turns out to be useful.
- Add debounce handling as part of the input event pipeline rather than inside sysfs polling.
- Use press duration as an input signal so a button can drive dimmer-style light control.

## Automation Model

- Add bindings from push button events to relay, motor, or cover actions.
- Add an automation handler that consumes semantic events from the shared event bus.
- Define a small action model so event handlers produce explicit commands instead of directly mutating hardware state.
- Add bindings that map short and long presses to different light actions such as toggle and dim.

## Actuator Model

- Introduce motors as first-class local actuators composed of two relays.
- Build covers on top of motors instead of wiring covers directly to relays.
- Track motor state separately from cover state when that improves restart and recovery behavior.

## Event System

- Extend the tagged-union event model with actuator and automation events.
- Add event tracing or structured logging to make event flows easier to debug.
- Keep integration communication as a star topology around the controller rather than a mesh of peer-to-peer channels.

## Transport And Integration

- Reintroduce MQTT as a thin external interface once local semantics are stable.
- Add Modbus RTU for inter-unit control after the local control loop is proven.
- Investigate GPIO v2 ABI over UniPi sysfs.
- Revisit Home Assistant auto-discovery once the external contract is stable.
- If Home Assistant MQTT auto-discovery is added, expose the MQTT-facing options already modeled by `nest` rather than inventing a separate HA-specific configuration layer.
- Expose an API for managing configuration so external tools can program controller state.
- Explore a Terraform provider that manages `nest` configuration through that API.

## Operations

- Add a dedicated `validate` command instead of only a `-validate` flag if the CLI grows.
- Add example configs for common unit roles such as input-only, relay-only, and local cover control.
- Decide how much configuration should be checked against live sysfs state at startup.
- Reproducible NixOS UniPi controller images.
- Expose a local web UI for controller inspection and configuration.
- Keep the web UI simple and server-driven, using something like `htmx` rather than a heavy frontend framework.
