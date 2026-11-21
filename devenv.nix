{ pkgs, ... }:

let
  homeAssistant = pkgs.home-assistant.override {
    extraComponents = [ "config" "frontend" "lovelace" "met" "mqtt" "recorder" ];
    extraPackages = ps: [ ps.home-assistant-frontend ];
  };
in
{
  languages.go = {
    enable = true;
    package = pkgs.go_1_26;
  };

  packages = with pkgs; [
    nushell
    dprint
    golangci-lint
    goreleaser
  ];

  services.mosquitto = {
    enable = true;
    bind = "127.0.0.1";
    port = 1883;
  };

  tasks."ha-smoke:prepare".exec = "${pkgs.nushell}/bin/nu deployments/local-ha-mqtt/prepare.nu";

  processes.home-assistant = {
    after = [ "ha-smoke:prepare" "devenv:processes:mosquitto@started" ];
    exec = ''
      export PYTHONPATH="${homeAssistant.pythonPath}"
      ${homeAssistant}/bin/hass --config "$DEVENV_STATE/ha-smoke/homeassistant"
    '';
  };

  processes.nest = {
    after = [ "ha-smoke:prepare" "devenv:processes:mosquitto@started" ];
    exec = ''
      go run ./cmd/nest --config "$DEVENV_STATE/ha-smoke/nest.yaml" controller_1
    '';
  };

  tasks = {
    "dev:format".exec = "dprint fmt";
    "dev:format-check".exec = "dprint check";
    "dev:test".exec = "go test -race ./...";
    "dev:lint".exec = "golangci-lint run ./...";
    "dev:vet".exec = "go vet ./...";
    "dev:build".exec = "go build ./...";
    "dev:artifacts".exec = "goreleaser release --snapshot --clean --skip=publish";
    "dev:check".after = [
      "dev:format-check"
      "dev:lint"
      "dev:vet"
      "dev:test"
      "dev:build"
    ];
  };
}
