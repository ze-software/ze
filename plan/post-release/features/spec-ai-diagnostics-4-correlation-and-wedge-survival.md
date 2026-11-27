# Spec: ai-diagnostics-4-correlation-and-wedge-survival

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli, plugin, config, docs |
| Priority | P3 |
| Release reason | Links operator commands, config reloads, bus events, and plugin RPCs with a shared operation ID and keeps core diagnostics reachable even if a control-plane lock wedges; existing shipped behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-ai-diagnostics-3-external-plugin-introspection.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Part 4 of the AI diagnostics architecture series. When a config reload or operator command triggers a failure across plugins, evidence is currently split across `show audit`, `show reload-status` (which stores only the last outcome string), `show log recent`, `show event recent`, and `show errors` with no shared correlation ID. In addition, MCP calls share the main command dispatcher, so a wedged lock can stall diagnostic queries.

This spec covers:
1. Propagating a lightweight `op-id` / `tx-id` across command, reload, and plugin RPC contexts so `slogutil`, `EventRing`, `report.Issue`, and `audit` attach it automatically, with `op <id>` filtering on log and event queries.
2. Expanding `reloadGeneration` (`internal/component/plugin/server/reload_generation.go`) into a bounded reload transaction journal (last 32 reloads with trigger, changed config paths, per-plugin verify/apply/rollback durations, and exact errors) exposed via `show reload history`.
3. Ensuring core diagnostic commands stay free of config/reload/reactor locks, enforcing bounded handler timeouts with explicit hung-handler diagnostics, and adding a daemon liveness watchdog that snapshots goroutines and flight-recorder traces automatically on stall.
