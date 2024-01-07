{
  description = "Basic flake for go";

  inputs = {
    # nixpkgs = { url = "github:NixOS/nixpkgs/6eed87d4490ce045dda1091999d1e38605afb8ea"; };
    nixpkgs = { url = "github:NixOS/nixpkgs/"; };
    flake-utils = { url = "github:numtide/flake-utils"; };
  };

  outputs = { self, nixpkgs, flake-utils, ... } :
    flake-utils.lib.eachDefaultSystem (system:
      let
        inherit system;
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShell = pkgs.mkShell {
          buildInputs = with pkgs; [
          go_1_20
          delve
          ];
        };
      });
}
