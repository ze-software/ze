# 165 — Reactor Service Separation

## Objective

Refactor the monolithic reactor into a hub/orchestrator model where `ze` is a protocol-agnostic hub and `ze bgp`, `ze rib`, `ze gr` are separate forked processes communicating via the 5-stage plugin protocol.

## Decisions

- Chose process-per-subsystem over in-process services: crash isolation, language freedom (BGP could be rewritten in Rust), per-process resource limits, independent debugging.
- All plugins are equal peers — no "built-in" vs "third-party" distinction; all use identical 5-stage stdin/stdout protocol.
- Two RIB packages coexist: `internal/plugin/bgp/rib/` for peer-to-peer route propagation (low-latency, inside `ze bgp`), and `internal/plugin/rib/` for Adj-RIB tracking and replay (separate `ze rib` process).
- Config routing uses longest-prefix match on handler paths: `bgp.peer.capability.graceful-restart` → ze gr; `bgp.peer.timers` falls through to `bgp` prefix → ze bgp.
- Hub does full config parsing (VyOS-style): parse entire file, validate against combined YANG schema, route JSON subtrees to correct processes.

## Patterns

- Plugin startup: fork → Stage 1 (declare schema + handlers) → Hub registers schema → Stage 2 (deliver config JSON subtrees) → Stages 3–5 (capabilities, registry, ready).
- CLI routing: `ze bgp peer list` connects to hub Unix socket, hub forwards by "bgp" prefix to ze bgp process.
- GR plugin has no root config block; it augments `bgp.peer.capability.graceful-restart` — hub longest-prefix routes only that subtree to ze gr.

## Gotchas

- The existing `SubsystemManager`, `plugin.Server`, `Dispatcher`, and `SchemaRegistry` are all reused by the hub — the separation is organisational (package location), not a new protocol.
- Spec was a planning document; the implementation summary section was left blank, suggesting much of this was designed but the actual implementation spanned many other specs.

## Files

- `internal/hub/` — hub orchestrator, process management, routing, config parsing
- `internal/plugin/bgp/` — BGP engine moved from `internal/bgp/` and `internal/reactor/`
- `cmd/ze/main.go`, `cmd/ze/bgp/main.go`
