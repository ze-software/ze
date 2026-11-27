# Spec: ai-diagnostics-3-external-plugin-introspection

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli, plugin, docs |
| Priority | P3 |
| Release reason | Closes the cross-process observability blind spot so an AI over MCP can inspect goroutines, profiles, logs, and runtime stats of forked external plugins identically to in-process plugins; existing shipped behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Part 3 of the AI diagnostics architecture series. Every Ze plugin must behave identically internal or forked (`ai/doors/system.md`), and `OnDoctorCheck` already crosses the process boundary. However, `show system goroutines`, `show system profile`, `show system memory`, and `show log recent` inspect only the daemon process. When a bug deadlocks, leaks memory, or spins CPU inside a forked plugin, an AI over MCP cannot see inside that process.

This spec covers:
1. Adding SDK-implemented runtime inspection callbacks in `pkg/plugin/sdk` and `ze-plugin-callback.yang` alongside `OnDoctorCheck` (`plugin-goroutines`, `plugin-profile`, `plugin-log-recent`, `plugin-runtime-stats`) with zero per-plugin boilerplate.
2. Extending `show system goroutines`, `show system profile`, and `show log recent` with an optional `plugin <name>` selector routing identically over `DirectBridge` (internal) and RPC (forked).
3. Adding a host-side fallback in `internal/component/plugin/process` reading `/proc/<pid>/status` and capturing a stack dump if the forked plugin's own RPC loop is wedged.
