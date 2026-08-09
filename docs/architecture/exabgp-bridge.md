# ExaBGP Bridge Plugin

The bridge runs an ExaBGP-API script against ze's BGP engine. It has two
runners with one behavior: an external SDK-mode process, and an internal
in-process runner.

<!-- source: internal/plugins/exabgp/bridgeplugin/internal.go -- runInternalBridge, familyDecls -->
<!-- source: internal/plugins/exabgp/bridgeplugin/config.go -- exabgp bridge config parsing -->

## The internal runner cannot read the `run` line

The external runner takes the script command from the process-manager `run`
line. A `RunEngine(conn net.Conn)` runner never sees that line, so the internal
runner reads the script command from the `exabgp { bridge { ... } }` config
root, delivered by the SDK `OnConfigure` callback at stage 2. This is the one
structural difference between the two runners; everything after it is shared.

## Stage ordering constrains the family declaration

Config is not available at stage-1 registration. `familyDecls` therefore
declares the CLI-default family at stage 1, and the `family` leaf refines the
ADD-PATH capability encoding at stage 3, once the config has arrived.

## Config root

The settings nest under a top-level `exabgp` root, on an owner decision of
2026-07-09, so the root stays available for later ExaBGP-related
configuration. The registry plugin name stays `exabgp-bridge`.
