---
kind: note
level:
stage:
---
Run `./le doc check verify` after editing any file under `docs/`, after adding or removing a plugin, or after touching a YANG `ze:command` declaration. `internal/le/doc/wiring.Verify` runs the native documentation drift, command-surface, and source-anchor checks and reports every finding.
