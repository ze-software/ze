# 157 — Hub Separation Phases (Master Overview)

## Objective

Refactor ZeBGP so `ze` is a hub/orchestrator that forks and coordinates plugins (`ze bgp`, `ze rib`, `ze gr`) communicating via pipes using the 5-stage protocol. This was a 7-phase effort.

## Decisions

- Config path passed to children via `--config` flag (not JSON config in stdin) — simpler and allows children to read their own config sections with existing parser.
- `internal/hub/` is a thin entry point composing existing `internal/plugin/` infrastructure (SubsystemHandler, SchemaRegistry, Hub) — avoids duplicating forking/pipe code.
- Live/Edit configuration model (VyOS-inspired): hub maintains two states, plugins query hub rather than hub pushing config. Pull model: `config verify` → plugin queries `query config live|edit` → plugin computes diff.
- Priority ordering for verify/apply (lower number first): BGP=100, RIB=200, GR=300, default=1000.
- GR plugin augments ze-bgp YANG (`ze-gr.yang`) rather than having GR config in `ze-bgp.yang` — clean separation of concerns.

## Patterns

- Binary detection: SubsystemHandler distinguishes full commands (with spaces, needing `--mode` flag) from binary paths.
- Child mode detection: `--child` flag takes precedence over environment variable and pipe detection to avoid ambiguity when user pipes config to standalone mode.

## Gotchas

- BGP code move (`internal/bgp/*` → `internal/plugin/bgp/*`) required updating 239 import references across 126 files. Budget 2-4 hours for this kind of wholesale package relocation.
- `internal/rib/` (BGP engine's peer-to-peer routing) and `internal/plugin/rib/` (adj-RIB tracking plugin) are DIFFERENT packages with similar names. The former moves with BGP; the latter is a separate process.

## Files

- `internal/hub/hub.go`, `config.go`, `orchestrator.go` — hub process entry point
- `cmd/ze/bgp/childmode.go` — BGP as hub child process with full 5-stage protocol + reactor integration
- `yang/ze-gr.yang` — GR augment of ze-bgp schema
