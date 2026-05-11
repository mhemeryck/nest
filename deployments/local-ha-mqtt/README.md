# Local Home Assistant MQTT

This deployment starts a disposable Home Assistant instance and Mosquitto broker for testing MQTT discovery behavior.
It is intended for local development only.

## Start

```sh
docker compose --project-directory deployments/local-ha-mqtt up
```

Home Assistant is available at <http://localhost:8123>.
Mosquitto is available on `localhost:1883`.
Both ports are bound to `127.0.0.1` so the stack is reachable from the local machine only.
This still allows a locally running `nest` process to publish to the test broker.

The first Home Assistant startup asks for a local user account.
After onboarding, add the MQTT integration from the Home Assistant UI.
Use `mosquitto` as the broker host and `1883` as the port.
The default Home Assistant MQTT discovery prefix is `homeassistant`.

## Publish A Test Light

Publish retained Home Assistant MQTT discovery config:

```sh
mosquitto_pub -h localhost -p 1883 -r -t 'homeassistant/light/nest_local_office_light/config' -m '{"name":"Office Light","unique_id":"nest_local_office_light","state_topic":"nest/units/local/lights/office_light/state","command_topic":"nest/units/local/lights/office_light/set","payload_on":"ON","payload_off":"OFF","availability_topic":"nest/units/local/availability","device":{"identifiers":["nest_local"],"name":"nest local","manufacturer":"nest"}}'
```

Publish retained availability and state:

```sh
mosquitto_pub -h localhost -p 1883 -r -t 'nest/units/local/availability' -m 'online'
mosquitto_pub -h localhost -p 1883 -r -t 'nest/units/local/lights/office_light/state' -m 'OFF'
```

The light should appear in Home Assistant after discovery is processed.
Use this to validate entity naming, unique ID behavior, availability, state payloads, and command topic behavior before encoding the contract in `nest`.

## Observe Commands

Subscribe to the test command topic before toggling the light in Home Assistant:

```sh
mosquitto_sub -h localhost -p 1883 -t 'nest/units/local/lights/office_light/set'
```

Home Assistant should publish `ON` or `OFF` to the command topic when the light is toggled.
`nest` should not subscribe to or act on these command topics until MQTT commands are explicitly enabled later.

## Cleanup

Stop the stack:

```sh
docker compose --project-directory deployments/local-ha-mqtt down
```

Remove persisted Home Assistant and Mosquitto state if you need a clean discovery test:

```sh
rm -rf deployments/local-ha-mqtt/homeassistant/.storage deployments/local-ha-mqtt/mosquitto/data/* deployments/local-ha-mqtt/mosquitto/log/*
```

To remove the retained discovery entity from a running broker, publish an empty retained message:

```sh
mosquitto_pub -h localhost -p 1883 -r -n -t 'homeassistant/light/nest_local_office_light/config'
```
