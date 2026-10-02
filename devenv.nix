{ pkgs, ... }:

{
  languages.go = {
    enable = true;
    package = pkgs.go_1_26;
  };

  packages = [
    pkgs.nushell
    pkgs.golangci-lint
    pkgs.goreleaser
    pkgs.openspec
  ];

  scripts = {
    nest-test.exec = "go test -race ./...";
    nest-lint.exec = "golangci-lint run ./...";
    nest-vet.exec = "go vet ./...";
    nest-build.exec = "go build ./...";
    nest-check = {
      package = pkgs.nushell;
      binary = "nu";
      exec = ''
        nest-lint
        nest-vet
        nest-test
        nest-build
      '';
    };
  };
}
