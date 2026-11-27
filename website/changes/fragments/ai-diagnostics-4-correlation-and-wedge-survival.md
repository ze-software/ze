---
spec: ai-diagnostics-4-correlation-and-wedge-survival
topic: Diagnostics
---

## Before

When a config reload or operator command triggered a fault across subsystems or plugins, evidence was split across `show audit`, `show reload-status` (which retained only the latest generation counter, outcome, and timestamp), `show log recent`, `show event recent`, and `show errors` with no shared correlation ID. Builtin command handlers ran with no default execution deadline or hung-handler diagnostics, and no always-on daemon liveness watchdog detected scheduler stalls or long-held control-plane locks to freeze pre-stall evidence.

## After

Every dispatched command and config reload now carries a process-scoped correlation ID (`operation-id`, formatted `op-<N>`) automatically attached to structured log entries, global bus events (including reload transaction `config` events), report-bus issues, audit entries, and reload records. `show log recent` (composable with `| plugin <name>`), `show event recent`, `show errors`, `show audit`, and `show system reload history` accept a command-owned `| operation <id>` pipe filter (`pipe: [{"name": "operation", "args": ["<id>"]}]` over MCP) to pull the complete cross-surface trail for one operation.

Reload inspection moves under `show system reload`: `show system reload status` replaces `show reload-status` while keeping its reload-fence semantics and adding `last-operation-id`, and `show system reload history [count <N>]` exposes a 32-entry leaf-locked reload transaction journal recording `generation`, `operation-id`, `trigger`, `outcome`, `duration-ms`, `changed-roots`, `tx-state`, per-plugin `verify-ms`/`apply-ms`/`rollback-ms`/`status`/`error` steps, and top-level `error`.

Core diagnostic commands never acquire `Server.txLock` or config-write locks, so they remain responsive during a wedged transaction. Operator/external builtin command handlers run under a bounded `60s` default timeout (`show system profile` declares `duration + 15s` via a generic `HandlerTimeout` hook; plugin-originated internal RPCs remain governed by their plugin lifecycle context): a handler that ignores cancellation past its deadline returns a structured timeout error naming `operation-id` and `ze show system goroutines mode blocked`, records into a 16-slot hung-handler ring, raises `doctor-command-handler-hung`, and auto-captures a flight-recorder trace. An always-on daemon liveness watchdog (`2s` tick, `5s` scheduler-drift threshold, `45s` `txLock` probe, active hung-handler probe, `30s` cooldown) freezes bounded goroutine stacks (`256 KiB` cap) and a flight-recorder snapshot into an 8-slot stall ring, raises `doctor-daemon-stall-detected`, and surfaces live status through `show system watchdog` and `show system triage` (`watchdog-running`, `stall-count`, `hung-handler-count`). `show skill ze-diagnostics` documents both workflows.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
