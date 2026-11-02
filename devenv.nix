{ pkgs, ... }:

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

  tasks = {
    "dev:format".exec = "dprint fmt";
    "dev:format-check".exec = "dprint check";
    "dev:test".exec = "go test -race ./...";
    "dev:lint".exec = "golangci-lint run ./...";
    "dev:vet".exec = "go vet ./...";
    "dev:build".exec = "go build ./...";
    "dev:check".after = [
      "dev:format-check"
      "dev:lint"
      "dev:vet"
      "dev:test"
      "dev:build"
    ];
  };
}
