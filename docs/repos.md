# Repositories Overview

## Summary

| Repository     | Language       | Purpose                                | Status      |
| -------------- | -------------- | -------------------------------------- | ----------- |
| `evok2mqtt`    | Python         | Bridge: Unipi websocket → MQTT         | Production  |
| `modbusbackup` | Python         | Direct RS-485 link for lighting        | Production  |
| `covers`       | Python         | MQTT cover controller (HA integration) | Legacy      |
| `hausmaus`     | Rust           | Next-gen controller (incomplete)       | Abandoned   |
| `nest`         | Go             | Next-gen controller PoC                | Development |
| `homelab`      | Terraform/YAML | Infrastructure (k3s, DNS, Unifi)       | Production  |

---

## evok2mqtt

**Purpose:** Bridge between Unipi's websocket API and MQTT broker.

**Architecture:**

```
Unipi evok websocket → evok2mqtt → MQTT → Home Assistant
MQTT command ← evok2mqtt ← Unipi evok websocket
```

**Key Features:**

- Subscribes to Unipi's websocket for real-time DI/DO/RO events
- Publishes state changes to MQTT topics
- Receives MQTT commands and forwards to Unipi websocket
- Single binary deployment

**MQTT Topics:**

- State: `{device_name}/{dev}/{circuit}/state`
- Command: `{device_name}/{dev}/{circuit}/set`

**Repo:** `github.com/mhemeryck/evok2mqtt`

---

## modbusbackup

**Purpose:** Direct RS-485/Modbus link for critical lighting (failsafe).

**Architecture:**

```
Unipi DI → evok → modbusbackup client → RS-485 → modbusbackup server → Relay
```

**Key Features:**

- Bypasses MQTT/Home Assistant for critical lighting
- Direct Modbus RTU communication
- YAML config maps inputs to outputs
- Works even when MQTT/HA is down

**Configuration:**

```yaml
- index: 0
  input: "2_03" # Digital input on input unit
  name: "kelder inkom inbouw"
  output: "2_01" # Relay on output unit
```

**Repo:** `github.com/mhemeryck/modbusbackup`

---

## covers

**Purpose:** MQTT-based cover/shade controller with Home Assistant integration.

**Architecture:**

```
MQTT command → covers.py → MQTT (relay commands) → evok2mqtt → Unipi RO
Unipi DI → evok2mqtt → MQTT (relay state) → covers.py → MQTT (cover state/position)
```

**Key Features:**

- Implements cover state machine (Open/Closing/Stopped/Closed)
- Position tracking (0-100%)
- Maps push button events to cover actions
- HA MQTT Cover integration support

**Limitations:**

- Depends on evok2mqtt for relay control
- Requires MQTT broker
- Not standalone from HA

**Status:** Legacy - to be replaced by `nest`

---

## hausmaus

**Purpose:** Next-gen Rust controller with direct sysfs access.

**Architecture:**

```
sysfs (Unipi) ← hausmaus → MQTT ← Home Assistant
```

**Key Features:**

- Direct sysfs access (no evok dependency)
- Rust for performance and safety
- MQTT pub/sub layer
- Support for PushButton, Light, Dimmer, Cover entities

**Current State:**

- sysfs read/write scaffolding complete
- MQTT client scaffolding complete
- Cover entity defined but methods are stubs
- **Not functional for covers**

**Status:** Abandoned - superseded by `nest` (Go)

---

## nest

**Purpose:** Next-gen Go controller PoC with direct sysfs access and MQTT integration.

**Architecture:**

```
sysfs (Unipi) ← nest → MQTT ← Home Assistant
                ↓
            Standalone operation
```

**Key Features:**

- Direct sysfs access (no evok dependency)
- Go for simplicity and fast development
- MQTT client for HA integration
- Device polling infrastructure
- Cover entity support (to be implemented)

**Current State:**

- `pkg/device/` - sysfs device management (DI/DO/RO)
- `main.go` - PoC with lights and push buttons
- MQTT layer - not yet added
- Cover controller - not yet implemented

**Todo for covers:**

- [ ] Add MQTT client (subscribe commands, publish state)
- [ ] Implement Cover entity with state machine
- [ ] Map push button inputs → cover commands
- [ ] Wire sysfs reads → MQTT state, MQTT commands → sysfs writes

**Repo:** `github.com/mhemeryck/nest`

---

## homelab

**Purpose:** Infrastructure as Code for home lab.

**Components:**

- `picl/` - Single-node Raspberry Pi k3s cluster
- `dns/` - DNS configuration
- `unifi-terraform/` - UniFi controller via Terraform

**Status:** Production

---

## Architecture Decision: nest vs hausmaus

**Why Go over Rust:**

- Faster development cycle
- Simpler dependencies (single `go.mod`)
- Easier to read/maintain
- Sufficient performance for this use case

**Why nest:**

- Direct sysfs access (standalone from evok)
- MQTT integration (HA compatible)
- Minimal dependencies
- Easy deployment (single binary)
