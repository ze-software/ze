---
kind: directive
level: MUST
stage:
---
**You MUST run `./le doc check verify` after editing any file under `docs/`, after adding or removing a plugin, and after touching a YANG `ze:command` declaration.** `internal/le/doc/wiring.Verify` runs the documentation drift, command-surface and source-anchor checks and reports every finding.
