# Home Automation Architecture

## Overview

Wired home automation system for a house, built with open-source components and a DIY approach.

**Guiding Principles:**

- **Wired**: Star topology to centralized electrical cabinet (reliable, no wireless interference)
- **Open**: Vendor-independent hardware and software
- **Local-first**: Works without internet; data owned locally
- **Low cost**: Mix and match DIY platforms (Raspberry Pi, Unipi)
- **Reproducible**: Automated provisioning and deployment

---

## High-Level Architecture

```
[Push Buttons] → [Unipi L303 (DI)] → [MQTT Broker] → [Home Assistant] → [MQTT] → [Unipi L403 (RO)] → [Relays] → [Lights/Covers]
                                           ↑
                     [Direct RS-485/Modbus Link for Lighting]
```

### Key Components

| Component             | Role                                             |
| --------------------- | ------------------------------------------------ |
| **Mosquitto**         | MQTT broker for event-based pub/sub              |
| **Home Assistant**    | Central automation engine, UI, entity management |
| **Unipi Neuron L303** | Digital inputs (64 DI) for push buttons          |
| **Unipi Neuron L403** | Relay outputs (56 RO) for lights                 |
| **Unipi Axon S605**   | DALI controller (future expansion)               |
| **evok2mqtt**         | Custom bridge: Unipi websockets → MQTT           |
| **modbusbackup**      | Custom bridge: Unipi websockets → RS-485/Modbus  |
| **nest**              | Next-gen standalone controller (Go)              |

### Communication Flow (via MQTT)

1. Push button pressed
2. Unipi DI detects change → evok websocket event
3. evok2mqtt publishes to MQTT state topic
4. MQTT broker forwards to subscribers
5. Home Assistant updates entity state, triggers automation
6. Home Assistant publishes to MQTT command topic
7. Unipi RO receives command, toggles relay
8. Light/Cover toggled, state update sent back

### Direct Link (Lighting Only)

For critical lighting (Nov 2021 update):

1. Push button pressed
2. Unipi DI → evok → modbusbackup client
3. Modbus message over RS-485 to output unit
4. modbusbackup server toggles relay directly
5. Light toggled

**Advantage**: Lighting works independently of MQTT/Home Assistant.

---

## Hardware Details

### Electrical Cabinet

- **Star topology**: All wires (lights, push buttons) converge to central cabinet
- **WAGO TOPJOB S terminal blocks**: Organize and distribute wiring
  - Two-level blocks for phase/neutral grouping
  - Orange spacers marks circuit boundaries
  - Jumpers for parallel connections

### Wiring

| Purpose      | Cable                             | Voltage     |
| ------------ | --------------------------------- | ----------- |
| Lights       | 3 x 1.5mm² (lights ≤16A)          | 240VAC      |
| High load    | 3 x 2.5mm² (≤20A)                 | 240VAC      |
| Push buttons | SVV 0.8mm² (signal cable)         | 24VDC       |
| DALI bus     | 2-wire (spare in cable runs)      | Low voltage |
| Covers       | 4 x 0.8mm² (up/down/stop/neutral) | 24VDC       |

### Unipi Neuron Units

- Raspberry Pi 3B+ base + I/O extension boards via SPI
- 24V power supply
- Digital inputs: 24V signaling
- Relay outputs: 240V switching
- RS-485 port for Modbus/daisy-chaining

**Models:**

- **L403**: 56 relay outputs (light/cover control)
- **L303**: 64 digital inputs (push button read-out)

---

## Software Stack

### Layers

```
[Home Assistant]           # Service layer (automations)
       ↑
[MQTT Broker]              # Network layer (Mosquitto)
       ↑
[evok2mqtt]               # Software layer (websockets → MQTT)
       ↑
[evok]                    # ModbusTCP/web APIs
       ↑
[Unipi Kernel]            # SPI polling to I/O boards
       ↑
[Raspberry Pi + I/O]      # Hardware layer
```

### Next-Gen Layers (nest)

```
[Home Assistant]           # Service layer (optional)
       ↑
[MQTT Broker]              # Network layer (Mosquitto)
       ↑
[nest]                    # Standalone controller
       ↑
[Unipi Kernel]            # SPI polling to I/O boards
       ↑
[Raspberry Pi + I/O]      # Hardware layer
```

---

## Future Considerations

- **DALI expansion**: Replace relay-based light control with DALI protocol
- **HA failover**: Run multiple Home Assistant instances
- **EMQX**: High-availability MQTT broker
- **Bus-based systems**: KNX for future wiring

---

## Useful Links

- Home Assistant: https://www.home-assistant.io/
- Unipi: https://www.unipi.technology/
- evok2mqtt: https://github.com/mhemeryck/evok2mqtt
- modbusbackup: https://github.com/mhemeryck/modbusbackup
- nest: https://github.com/mhemeryck/nest
