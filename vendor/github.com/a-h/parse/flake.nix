{
  description = "parse";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    version = {
      url = "github:a-h/version";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      nixpkgs,
      version,
      ...
    }:
    let
      allSystems = [
        "x86_64-linux" # 64-bit Intel/AMD Linux
        "aarch64-linux" # 64-bit ARM Linux
        "aarch64-darwin" # 64-bit ARM macOS
      ];
      forAllSystems =
        f:
        nixpkgs.lib.genAttrs allSystems (
          system:
          f {
            inherit system;
            pkgs = import nixpkgs {
              inherit system;
              overlays = [
                (final: prev: {
                  version = version.packages.${system}.default; # Used to apply version numbers to the repo.
                })
              ];
            };
          }
        );
    in
    {
      # `nix develop` provides a shell containing development tools.
      devShell = forAllSystems (
        { pkgs, ... }:
        pkgs.mkShell {
          buildInputs = [
            pkgs.go_1_27
            pkgs.gopls
            pkgs.version
            pkgs.xc
          ];
        }
      );
    };
}
