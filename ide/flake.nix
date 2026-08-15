{
  description = "IDE dev shell and container runtime";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          idehost-runtime = pkgs.buildEnv {
            name = "idehost-runtime";
            paths = with pkgs; [
              bashInteractive
              coreutils
              wget
              cacert
              nix
              git
              curl
            ];
            pathsToLink = [ "/bin" "/etc" "/share" ];
          };

          default = self.packages.${system}.idehost-runtime;
        });

      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              bun
              podman
              nix
              git
              wget
              curl
            ];
          };
        });
    };
}
