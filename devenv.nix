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
    openspec
  ];

  scripts = {
    nest-format.exec = "dprint fmt";
    nest-format-check.exec = "dprint check";
    nest-test.exec = "go test -race ./...";
    nest-lint.exec = "golangci-lint run ./...";
    nest-vet.exec = "go vet ./...";
    nest-build.exec = "go build ./...";
    nest-check = {
      package = pkgs.nushell;
      binary = "nu";
      exec = ''
        nest-format-check
        nest-lint
        nest-vet
        nest-test
        nest-build
      '';
    };
  };
}
