# 147 — API Command Restructure Step 7: RIB Namespace & Plugin Commands

## Objective

Create the `rib` namespace with introspection commands and determine correct ownership of cache/RIB commands between engine and plugins.

## Decisions

- Kept `msg-id`, `forward`, and `rib show/clear` as engine builtins, not plugin-provided. The spec's goal of making them plugin-provided was architecturally flawed: cache is managed by the reactor (engine), plugins communicate via IPC and cannot directly access engine state. Plugin-provided commands would create circular command routing.
- Engine builtins work regardless of plugin presence, which is the correct behavior for cache management.
- `rib event list` returns 4 events (`cache`, `route`, `peer`, `memory`) per ipc_protocol.md — not just 2 as planned.

## Patterns

- RIB plugin correctly declares its own commands (`rib adjacent *`) via Stage 1 `declare cmd` — those ARE plugin-provided. The distinction is: commands that operate on engine state = builtins; commands that operate on plugin state = plugin-provided.

## Gotchas

- Spec assumed cache commands could be plugin-provided because "they're related to RIB functionality." Wrong. Ownership follows where the state lives, not conceptual grouping. The msg-id cache is in the reactor, so the commands belong in the engine.

## Files

- `internal/component/plugin/handler.go` — added RIB introspection handlers via `RegisterRibHandlers()`
- `internal/component/plugin/rib/rib_test.go` — startup protocol tests added
