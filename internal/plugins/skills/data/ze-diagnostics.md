---
name: ze-diagnostics
description: Diagnose Ze config errors and debug live runtime issues, hangs, drift, and bad packets.
---

# Ze Diagnostics

Use this when Ze config fails to parse or validate, or when debugging a misbehaving Ze daemon at runtime (crashes, hangs, slow reloads, plugin wedges, dataplane drift, unexpected drops, or malformed wire frames).

## Commands

```sh
ze cli -c "show config validation <file> | json"
ze show diagnostic [--json] <diagnostic-code>
ze config fix --plan <file>
```

## Diagnostic Shape

Fields from `show config validation | json`:

- `code`: stable lower-kebab diagnostic code
- `severity`: `error` or `warning`
- `message`: short human summary
- `path`, `line`, `column`, `length`: source span
- `expected` and `actual`: mismatch facts when available
- `help`: concise next action
- `fix-safety`: safety label for a repair
- `repair`: optional repair id and summary
- `related`: extra spans or facts

## Fix Safety

- `format-only`: formatting only
- `section-local`: confined to one config section
- `behavior-preserving`: no runtime behavior change
- `api-changing`: config API shape may change
- `target-changing`: runtime target may change
- `requires-human-review`: cannot prove the edit is safe

## Agent Triage

1. Run the failing command with `--json`.
2. Use the span to inspect the relevant config.
3. Run `ze show diagnostic <code>` before broad changes.
4. Fix the earliest root cause first.
5. Re-validate after each fix.

## Live Runtime Triage

When a running Ze daemon misbehaves, start with one structured call before issuing individual queries.

### 1. Capture One Triage Snapshot

Over CLI:

```sh
ze cli -c "show system triage | json"
```

Over MCP, call `ze_show_system` with `action: "triage"` (or `ze_execute` with `"show system triage"`), or discover and read the read-only `ze://triage` resource via `resources/list` and `resources/read` (scope-checked against `"show system triage"`, 5s live-state TTL).

One round-trip returns build provenance (`version`), component `health`, daemon `doctor` findings, recent `errors`, active `warnings`, saved `crashes`, unhealthy `plugins`, `reload` status (including `last-operation-id` when a reload has run), `recent-logs` (warn and error), and process `runtime` vitals (including `watchdog-running`, `stall-count`, `hung-handler-count`, and `bad-packet-count`). Sections whose runtime source is unreachable report `available: false` with a reason; capped lists report `total-count` and `truncated` so you know when to fetch full details.

### Symptom-to-Command Routing

Pick the drill-down matching what `show system triage` (or the operator's symptom) reports:

| Symptom | First command(s) |
|---|---|
| Unknown failure — start here | `show system triage` (or MCP `resources/read` `ze://triage`) |
| Crash or recovered panic (`doctor-internal-defect`) | `show crashes name <file>` |
| Command timed out or daemon feels wedged | `show system watchdog`, then `show system goroutines mode blocked` and `show system trace id <id>` |
| Failed or slow config reload | `show system reload history`, `show system reload status`, then correlate with `\| operation <id>` |
| Reconstruct one incident across surfaces | `\| operation <id>` on `show log recent`, `show event recent`, `show errors`, `show audit`, `show system reload history` |
| High CPU, memory growth, or lock contention | `show system profile type <cpu\|heap\|allocs\|mutex\|block\|goroutine> \| summary` |
| One plugin (internal or forked) misbehaving or stuck | Add `\| plugin <name>` to `show system goroutines`, `show system profile`, `show runtime memory`, `show system memory`, or `show log recent` |
| Intended config diverges from kernel/dataplane (`doctor-dataplane-drift`) | `show system drift` or `show system drift \| subsystem <name>` |
| Packet dropped or accepted unexpectedly by firewall | `show firewall trace table <t> [chain <c>] ...` |
| BGP route missing, wrong best path, or policy drop | `show bgp rib best \| reason` and `show policy test peer <sel> <direction> update <hex>` |
| Malformed BGP wire frame or protocol panic reproducer | `show system bad-packets \| protocol bgp` |
| Portable live diagnostic snapshot for escalation | `request support archive module log-ring,report,health,watchdog,bad-packets,drift,goroutines,event-ring,plugins,reload-history` |

### Agent Rules (CLI and MCP)

- **Always pass structured `pipe` arrays over MCP**: never embed `"| summary"` or `"| plugin bgp"` inside a command argument string. Pass `pipe: [{"name": "summary"}]` (no `"args"` key when argumentless) or `pipe: [{"name": "plugin", "args": ["bgp"]}]` (exact `"args"` string array for value pipes). Multiple pipes compose in order in the same array.
- **Trust honesty flags before concluding**: check `available: false` (unreachable source — not an empty healthy list), `truncated: true` (more records exist — narrow or raise `count`), `incomplete: true` / `unevaluated: true` on `show firewall trace` (kernel-only predicate could not be decided offline), and `fallback: true` on `| plugin <name>` (forked plugin did not answer RPC; inspect `fallback-reason`, `host`, and `stderr-tail`).
- **Diagnostic reads survive reload wedges**: `show system triage`, `show system watchdog`, `show system reload status`, and `show system reload history` read only leaf locks/atomics without taking `Server.txLock` — run them even when config reloads or other commands time out.
- **Remote `request support archive` saves to the daemon store**: over MCP/SSH/REST/gRPC, never pass `--output`; read the returned `archive-key` with `show data cat <key>`.

### 2. Drill Down Based on What Triage Flags

- **Crashes and recovered panics (`doctor-internal-defect`)**: inspect the saved crash report named in `crashes` or the error `fix` field with `show crashes name <file>`.
- **Report-bus errors and warnings**: fetch full lists with `show errors` or `show warnings`, and explain any unfamiliar code with `ze show diagnostic <code>`.
- **Logs and event history**: query `show log recent level err` (reads the dedicated fault ring, immune to `info` bursts; also supports `level warn`, `component <name>`, and `count <N>`), `show event recent` (includes `source`, `subject`, `direction`, and `operation-id` when present), and `show event delivery`.
- **Cross-surface correlation by operation ID (`| operation <id>`)**: every dispatched operator command and config reload carries a process-scoped correlation ID (`op-<N>`) across logs, bus events, report-bus issues, audit records, and reload history. To reconstruct one incident across every surface:
  1. Grab the `operation-id` from `show system reload history [count <N>]`, `show system reload status` (`last-operation-id`), `show audit`, `show errors`, or a command timeout error.
  2. Pull the correlated trail across surfaces with `| operation <id>`:
     - `show log recent | operation <id>` (composable with `| plugin <name>`, e.g. `show log recent | plugin <name> | operation <id>`)
     - `show event recent | operation <id>`
     - `show errors | operation <id>`
     - `show audit | operation <id>`
     - `show system reload history | operation <id>`
  3. Over MCP, pass structured pipe entries with the `"args"` array syntax: `pipe: [{"name": "operation", "args": ["<id>"]}]` (or `pipe: [{"name": "plugin", "args": ["<name>"]}, {"name": "operation", "args": ["<id>"]}]` on `show log recent`).
- **Reload failures and slow transactions (`show system reload history`)**: run `show system reload history [count <N>]` (retains the newest 32 reloads under a leaf lock without taking `txLock`) to inspect `generation`, `operation-id`, `trigger`, `outcome`, `duration-ms`, `changed-roots`, `tx-state`, per-plugin `verify-ms`/`apply-ms`/`rollback-ms`/`status`/`error`, and top-level `error`. Pair with `show system reload status` (`last-operation-id`), `show doctor`, and `show plugin list`.
- **Wedged handlers and daemon stalls (`show system watchdog`, `doctor-command-handler-hung`, `doctor-daemon-stall-detected`)**: core diagnostic commands read only leaf locks/atomics and stay reachable even while `Server.txLock` is held. Operator command handlers run under a bounded `60s` default timeout (`show system profile` extends to `duration + 15s`); a handler that ignores cancellation returns a structured timeout error naming `operation-id` and `ze show system goroutines mode blocked`, raises `doctor-command-handler-hung`, and auto-captures a flight-recorder trace. The always-on liveness watchdog (`2s` tick, `5s` scheduler-drift threshold, `45s` `txLock` hold probe, active hung-handler probe, `30s` cooldown) captures bounded goroutine stacks (`256 KiB` cap) + a flight-recorder trace and raises `doctor-daemon-stall-detected`. When a command times out or `show system triage` reports `stall-count > 0` or `hung-handler-count > 0`:
  1. Run `show system watchdog` to inspect `running`, `ticks`, `stall-count`, probe health (`config-tx-lock`, `command-handlers`), `hung-handlers`, and `recent-stalls` (including captured `stacks` and any `trace-snapshot-id`).
  2. Run `show system goroutines mode blocked` (use `mode full` only if needed) to inspect live waiting goroutines.
  3. Fetch any auto-captured pre-stall execution trace named in `trace-snapshot-id` via `show system trace id <id>`.
  4. Correlate a hung command's `operation-id` with `show log recent | operation <id>`, `show errors | operation <id>`, and `show audit | operation <id>`.
- **Hangs, deadlocks, or memory growth**: inspect waiting goroutines with `show system goroutines mode blocked` (use `mode full` only for a hang), and capture AI-readable top-function JSON profile summaries without `go tool pprof` using `show system profile type heap | summary`, `type cpu duration 10s | summary`, `type mutex | summary`, `type block | summary`, `type goroutine | summary`, or `type allocs | summary` (over MCP, pass `pipe: [{"name": "summary"}]`). Bare `show system profile` without `| summary` returns raw `pprof-base64` bytes for `go tool pprof`.
- **Pre-incident execution traces**: list auto-captured and manual Go flight-recorder snapshots with `show system trace`, capture one immediately with `request system trace snapshot`, and fetch a snapshot's trace bytes with `show system trace id <id>`.
- **Forked and internal plugin runtime introspection (`| plugin <name>`)**: scope any of the five runtime diagnostic commands to a specific running plugin process without shell access:
  - `show system goroutines mode blocked | plugin <name>` (also `mode summary` or `mode full`; runs on an independent SDK worker so it still answers when the plugin's ordinary `eventLoop` is deadlocked)
  - `show system profile type heap | plugin <name> | summary` (composable in either order with `| summary` across `cpu`, `heap`, `goroutine`, `allocs`, `mutex`, and `block`)
  - `show runtime memory | plugin <name>` (Go `MemStats`, goroutines, CPU count, `GOMAXPROCS`, Go version)
  - `show system memory | plugin <name>` (Linux `/proc/<pid>/status`, `wchan`, and open `fd-count`; in-daemon plugins report `shared-daemon-process: true`)
  - `show log recent level err | plugin <name>` (queries the plugin's own process-local `slogutil` ring; also supports `level`, `component`, and `count`)
  - Over MCP, pass structured pipe entries with the exact `"args"` list syntax: `pipe: [{"name": "plugin", "args": ["<name>"]}]` (or `pipe: [{"name": "plugin", "args": ["<name>"]}, {"name": "summary"}]` on `show system profile`).
  - **Interpreting responses**: every answer includes `plugin`, `process-mode` (`"internal"` for in-daemon goroutines or `"external"` for forked subprocesses), and `pid`. When a forked external plugin is wedged, timed out, disconnected, or non-Go, the daemon never sends `SIGQUIT` or any terminating signal; instead it returns `fallback: true` with `fallback-reason` (`"timeout"`, `"unsupported"`, or `"transport-error"`), `rpc-error`, `host` Linux `/proc/<pid>` metrics (`state`, `wchan`, `threads`, `vm-rss-kb`, `fd-count`), and up to 32 recent `stderr-tail` lines.
- **Dataplane drift (`show system drift`, `doctor-dataplane-drift`)**: detect silent divergence between Ze's intended state and live programmed kernel/dataplane state across both internal and forked plugins (`firewall`, `fib-kernel`, `interface`):
  - Run `show system drift` for all registered subsystems, or narrow to one subsystem with `show system drift | subsystem <name>` (e.g., `show system drift | subsystem firewall`).
  - Over MCP, pass `pipe: [{"name": "subsystem", "args": ["firewall"]}]`.
  - `show doctor` runs the runtime `dataplane-drift` check and surfaces drifted or errored subsystems under `doctor-dataplane-drift` with fix `ze show system drift`.
- **Decision-trace suite (firewall and BGP)**:
  - **Firewall packet trace**: evaluate a synthetic packet read-only against `firewall.LastApplied()` (works across kernel and VPP without touching nftables/VPP) with `show firewall trace table <t> [chain <c>] [protocol tcp|udp|icmp|icmpv6] [source-address <ip>] [destination-address <ip>] [source-port <1..65535>] [destination-port <1..65535>] [input-interface <if>] [output-interface <if>] [connection-state new|established|related|invalid]`. The response returns per-term `steps`, `verdict`, `matched-chain`, `matched-term`, and `default-policy`; kernel-only match predicates report `unevaluated: true` and `incomplete: true` instead of guessing.
  - **BGP explain**: inspect BGP best-path selection steps with `show bgp rib best | reason` and dry-run import/export filter chains without mutating sessions via `show policy test peer <sel> <direction> update <hex>`.
- **Bad-packet reproducers (`show system bad-packets`)**: inspect the always-on 16-slot x 4 KiB circular ring of malformed wire frames and task-panic frames (captured on BGP RFC 7606 actions, BGP header/length parse errors, and recovered `taskguard` panics):
  - Check `show system triage` `runtime.bad-packet-count` to see whether any offending frames were recorded.
  - Fetch frames newest-first with `show system bad-packets [count <N>]` (`count` `1..16`, default `16`) or filter by protocol with `show system bad-packets | protocol bgp` (over MCP: `pipe: [{"name": "protocol", "args": ["bgp"]}]`). Each entry returns `id`, `captured-at`, `protocol`, `peer`, `message-type`, `action`, `reason`, `length`, `truncated`, and lowercase `hex` payload suitable for test reproducers.
- **Live support bundle modules (`request support archive`)**: alongside the 20 static modules, collect bounded live daemon runtime modules in one archive:
  - `request support archive module log-ring,report,health,watchdog,bad-packets,drift,goroutines,event-ring,plugins,reload-history`
  - Over the daemon route (CLI/SSH/MCP/REST/gRPC), the archive is saved under `support/archive/<name>` and retrieved with `show data cat <key>`.

### 3. Bug Report Essentials

Include these facts from triage and drill-down when reporting a runtime bug:

- Exact `version` identity (`release`, `build-date`, `commit`, `modified`, `go`, `os`, `arch`, and `image` when present)
- Failing `doctor` and `errors` codes (including `doctor-internal-defect` and `doctor-dataplane-drift`) and their messages
- Relevant `crashes` file name and stack trace when any crash or recovered panic occurred
- Captured `bad-packets` `hex` payload and metadata when malformed wire input or a protocol panic was involved
- Recent warn and error log entries and relevant `show event recent` history
- Concrete steps and config that triggered the behavior
