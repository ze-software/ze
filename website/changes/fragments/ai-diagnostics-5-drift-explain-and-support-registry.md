---
spec: ai-diagnostics-5-drift-explain-and-support-registry
topic: Diagnostics
---

## Before

Silent intent-vs-actual dataplane drift was checked only by `firewall.AuditTables()` for warning management, with no unified read-only drift query or checks for `fib-kernel` and `interface` across internal and forked plugins. Firewall rules had no offline decision-trace command beside BGP's `show bgp rib best | reason` and `show policy test`. Malformed BGP wire frames and recovered panic frames were logged as summaries and discarded rather than retained as raw hex reproducers. `request support archive` used a hardcoded 20-module map that omitted live daemon runtime rings, and MCP `resources/read` served only static `ui://` embedded assets with a 1-hour cache TTL.

## After

A new read-only dataplane drift registry (`internal/core/drift`) and generic plugin callback (`ze-plugin-callback:drift-check`, Stage 1 `drift-subsystems`) check `firewall`, `fib-kernel`, and `interface` identically across internal and forked plugins. `show system drift` (narrowable with `| subsystem <name>`, MCP `pipe: [{"name": "subsystem", "args": ["firewall"]}]`) returns per-subsystem status and bounded findings (`64` per subsystem, `5s` timeout), and `show doctor` surfaces drifted or errored subsystems under `doctor-dataplane-drift` (fix `ze show system drift`).

`show firewall trace` (`ze-firewall:show-trace`) evaluates a synthetic packet read-only against `firewall.LastApplied()` across both kernel and VPP backends, reporting per-term match steps, jumps/gotos, actions, and final verdict while marking kernel-only predicates explicitly as `unevaluated: true` and `incomplete: true`.

An always-on 16-slot x 4 KiB circular ring (`internal/core/badpacket`, 64 KiB max total memory, zero happy-path allocations) captures malformed BGP RFC 7606 UPDATEs, BGP header/length parse errors on both standard and coalesced read paths, and `taskguard` panic frames from owned read buffers. `show system bad-packets [count <N>]` (narrowable with `| protocol <name>`) returns newest-first lowercase hex reproducers, and `show system triage` reports `runtime.bad-packet-count`.

`request support archive` now uses the registrable `internal/core/supportreg` leaf registry and adds 10 bounded live runtime modules (`log-ring`, `report`, `health`, `watchdog`, `bad-packets`, `drift`, `goroutines`, `event-ring`, `plugins`, `reload-history`) alongside the 20 existing static modules. MCP `resources/list` and `resources/read` now expose read-only `ze://triage` (authorized via `checkCommandScope("show system triage")`, stamped by single-site `stampCacheHints` with a 5-second live-state TTL while `ui://` assets keep 1 hour). `show skill ze-diagnostics` documents all of these workflows.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
