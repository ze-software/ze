# Spec: ai-diagnostics-5-drift-explain-and-support-registry

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli, plugin, docs |
| Priority | P3 |
| Release reason | Enables an AI over MCP to detect silent intent-vs-actual dataplane divergence, trace policy/routing decisions, pull offending bad wire packets as test reproducers, and collect live daemon snapshots via registrable support modules; existing shipped behavior and its obligations remain required. |
| RFC gaps | - |
| Depends | spec-ai-diagnostics-4-correlation-and-wedge-survival.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-27 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Part 5 of the AI diagnostics architecture series. Subtle networking bugs rarely panic: they silently diverge between intended in-memory state and programmed kernel/VPP state, or drop routes/packets in a filter pipeline without obvious errors. Meanwhile, `internal/component/support/modules.go` uses a hardcoded central `moduleRegistry` map that omits live daemon runtime state (`log-ring`, `report`, `event-ring`, `plugins`, `reload-history`, `health`, `goroutines`).

This spec covers:
1. Standardizing a read-only dataplane drift-check pattern across writers (kernel FIB, VPP, interfaces, firewall/nft, IPsec/XFRM, MPLS) exposed through `show doctor` and per-subsystem `show <subsystem> drift` commands.
2. Adding read-only decision-trace (`explain`) commands for BGP route selection/export and firewall rule evaluation that replay the production evaluator and return structured step-by-step verdicts.
3. Adding a tiny always-on "last bad packets" ring across protocol engines (storing the last 16 frames that triggered a parse error, RFC 7606 action, or `taskguard` panic) so an AI can extract exact reproducer bytes over MCP.
4. Replacing the hardcoded `moduleRegistry` map in `internal/component/support/modules.go` with a registration seam so live daemon subsystems contribute bounded snapshot modules to `request support archive` (and aligning with `plan/post-release/features/spec-mcp2026-5-state-resources.md` for `ze://triage`).
