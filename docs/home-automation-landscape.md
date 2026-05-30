# Home Automation Landscape

This note captures the broader home automation context that informs `nest`'s distributed control model.
It is intentionally high-level and directional rather than a full protocol comparison.

## Professional Wired Systems

Professional wired systems usually separate semantic behavior from low-level device addressing.
They are designed for reliable building control rather than consumer retrofit convenience.

### KNX

KNX is a decentralized building automation bus.
Devices communicate through group addresses and can exchange commands, events, and status feedback.

KNX installations can be fully distributed, centrally supervised, or a mix of both.
A button can send a bus telegram, an actuator can react to it, and a visualization or controller can observe the same bus state.

This is relevant to `nest` because it shows that bindings do not have to imply direct source-to-target commands.
A source can publish an event or intent, and the interested target-side participant can act using its own local state.

### DALI

DALI is primarily a lighting bus.
It is focused on luminaires, drivers, groups, scenes, dimming, and lighting-specific control.

DALI is not a whole-house automation system by itself in the same way KNX is.
It is still useful as a future actor because lighting control has richer semantics than simple relay on/off.

For `nest`, DALI should fit below the semantic light model as another actor capability.
The light entity should not need to know whether it is backed by sysfs, DALI, Modbus, or another transport.

## Industrial And Control Buses

Industrial protocols often expose registers, coils, messages, or IO points rather than home-automation entities.
They are robust and practical, but the semantic layer usually has to be built above them.

### Modbus

Modbus RTU has a clear master/slave request-response shape.
A slave does not spontaneously publish events to the bus.

That makes Modbus different from pub/sub systems such as MQTT.
However, a Modbus master write can still be used as an event signal to a target-side unit.
The existing `modbusbackup` setup uses this pattern: a source-side input event writes a coil, and the target-side unit treats that write as a trigger to apply local state-aware behavior.

For `nest`, Modbus can support at least two actor models:

- event signal writes, where a coil write represents a source event or trigger
- concrete command writes, where a coil or register represents actuator state or a direct command

The semantic binding model should not force one interpretation globally.

### CAN And Similar Buses

CAN-style systems are message-oriented and can support distributed control well.
They are common in embedded and industrial systems rather than conventional home automation installations.

For a future custom hardware direction, a CAN-like or bus-like design could be a good fit if it supports semantic event and state distribution cleanly.

## Consumer IoT Systems

Consumer home automation is usually retrofit-driven.
It commonly uses Zigbee, Z-Wave, Wi-Fi, Bluetooth, vendor clouds, and bridge devices.

Home Assistant is strongest in this space because it normalizes many unrelated integrations into one semantic entity and automation model.
Its automations generally target entities and services, not transport routes.
The underlying integration decides how a light, switch, cover, sensor, or lock is reached.

This is relevant to `nest` because it reinforces a useful split:

```text
binding or automation = semantic intent
integration or actor = transport-specific reachability and execution
```

## Matter And Thread

Matter is an application-layer standard for consumer smart-home interoperability.
It can run over Ethernet, Wi-Fi, or Thread.

Thread is a low-power wireless mesh network often used with Matter devices.
Matter over Ethernet is possible, but Matter is still an IP-based smart-device protocol rather than a KNX/DALI-style field bus.

Matter is important for consumer interoperability, but it should not drive `nest`'s internal architecture.
For `nest`, Matter would be best treated as a future integration actor or external ecosystem bridge.

## Implications For Nest

`nest` should use a semantic model above transport-specific actor details.
The same binding should be able to describe local sysfs control, MQTT-backed distributed behavior, Modbus-backed event delivery, or future bus-based hardware.

The model should keep these layers separate:

```text
semantic entities
  -> bindings and actions
  -> actor capability or execution strategy
  -> transport-specific commands, events, state, or addresses
```

Bindings should stay transport-independent.
They describe what semantic relationship should exist, not how a transport carries it.

Actor capabilities should decide execution strategy.
Examples include:

- local execution for same-unit entities
- pub/sub event replication for MQTT-like transports
- event signal writes for Modbus-style trigger delivery
- concrete command writes for direct actuator control
- future bus-native group, scene, or state feedback behavior for lighting buses such as DALI

This lets `nest` unify the existing MQTT and Modbus setups while leaving room for future custom distributed hardware.
