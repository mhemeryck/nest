let state = ($env.DEVENV_STATE | path join ha-smoke)
let home_assistant = ($state | path join homeassistant)
let sysfs = ($state | path join sys)
mkdir $home_assistant

if not ($sysfs | path exists) {
    cp -r test/fixtures/sys $sysfs
}

{
    config: null
    frontend: null
    lovelace: null
    recorder: null
    http: {server_host: "127.0.0.1", server_port: 8123}
} | to yaml | save --force ($home_assistant | path join configuration.yaml)

let broker = {
    host: "127.0.0.1"
    port: ($env.MOSQUITTO_PORT | into int)
    client_id: "nest-ha-smoke"
    topic_prefix: "nest"
}

open test/fixtures/config.covers.yaml
| upsert actors.mqtt.broker $broker
| upsert units.controller_1.actors.mqtt.enabled true
| update units.controller_1.actors.sysfs.root $state
| update cover_control.max_run "5s"
| to yaml
| save --force ($state | path join nest.yaml)
