# 161 — Hub Phase 4: BGP Process Separation

## Objective

Move BGP code (`internal/bgp/*`, `internal/rib/`, `internal/reactor/`) under `internal/component/bgp/` and make `ze bgp` work as a forked child process of the hub completing the 5-stage protocol.

## Decisions

- `internal/rib/` (BGP engine's peer-to-peer zero-copy routing) moves WITH BGP to `internal/component/bgp/rib/`. It is NOT the same as `internal/component/plugin/rib/` (adj-RIB tracking plugin). Both names are similar but are completely different concerns.
- Child mode detection precedence: `--child` flag > `ZE_CHILD_MODE=1` env var > pipe detection. Explicit flag prevents ambiguity when user pipes config to standalone mode.
- `ze bgp` in child mode completes 5-stage protocol declaring ze-bgp schema and handlers, then loads the BGP reactor from the config file after receiving `ready`.
- Implementation Summary left blank — actual implementation tracked in 157-hub-separation-phases.

## Patterns

None beyond child mode detection precedence.

## Gotchas

- The BGP code move required updating 239 import references across 126 files. Any wholesale package relocation of this scale should budget 2-4 hours for mechanical import updates + test verification.
- `internal/rib/` vs `internal/component/plugin/rib/` naming collision risk: these are easy to confuse. The former is BGP-engine-internal (zero-copy), the latter is a separate adj-RIB process.

## Files

- `internal/component/bgp/` — all BGP protocol code (moved from `internal/bgp/`, `internal/rib/`, `internal/reactor/`)
- `cmd/ze/bgp/childmode.go` — child mode entry point with 5-stage protocol
