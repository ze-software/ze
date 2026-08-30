# API Commands

**Source:** ExaBGP `reactor/api/command/`, `reactor/api/dispatch/`
**Purpose:** Document all API commands for compatibility

---

## Overview

Ze uses verb-first command paths with JSON or text encoding.

### Transport Request Metadata

REST and gRPC remain thin adapters over the shared API engine. `Execute` and
`Stream` pass the caller's `context.Context` together with trusted auth
metadata (`Username`, `RemoteAddr`) into the engine; hub wiring then builds
`pluginserver.CommandContext` for dispatcher/accounting use.

Request bodies and plugin RPC payloads do not carry identity fields. Caller
identity is injected only by trusted transport wiring.

### ExaBGP Differences

| Aspect | ExaBGP | Ze |
|--------|--------|-------|
| Syntax styles | v4 (action-first) and v6 (target-first) | Verb-first only |
| Encoder | json or text (v4), json only (v6) | json or text |
| Peer selectors | `*`, IP, filters (`[local-as ...]`) | `*`, IP, negated (`!IP`) |
| Multi-session filters | Supported (draft) | Not supported |
| Forward command | Not available | `request cache forward <id> <selector>` for route reflection |

<!-- source: internal/component/plugin/server/command.go -- Dispatcher.Dispatch, IsReadOnlyPath -->

---

## Command Verb Taxonomy

Commands follow a **verb-first** convention: `<action> <module> [args...]`.
The action verb determines the command's behavior; the module implements it.

| Verb | Purpose | Examples |
|------|---------|---------|
| `show` | Read-only display (returns data, exits) | `show bgp peer <selector> detail`, `show warnings` |
| `set` | Create or modify | `set bgp peer X ...` |
| `delete` | Remove | `delete bgp peer X` |
| `update` | Route operations (announce, withdraw, refresh), firmware, prefix data | `update system firmware check`, `update bgp peer * prefix` |
| `monitor` | Long-running auto-refreshing display | `monitor bgp` (TUI dashboard) |

<!-- source: internal/component/cmd/show/doc.go -- show verb -->
<!-- source: internal/component/cmd/set/doc.go -- set verb -->
<!-- source: internal/component/cmd/delete/doc.go -- delete verb -->
<!-- source: internal/component/cmd/update/doc.go -- update verb -->

**Internal dispatch:** `<action> <module>` is dispatched as the module implementing the action.
For example, `monitor bgp` is handled by the BGP module's monitor implementation.
Legacy noun-first RPCs (`peer list`, `bgp summary`) remain for internal dispatch but
user-facing commands use verb-first syntax.

**Streaming vs polling:** `monitor` commands keep the display active and auto-refresh.
`monitor event` streams live events line-by-line. `monitor bgp` polls summary data
every 2 seconds and renders a dashboard. Both use the `monitor` verb because they
produce continuously-updating output.

## Command Categories

| Category | Commands |
|----------|----------|
| Daemon | shutdown, reload, restart, status |
| Session | ack, sync, reset, ping, bye |
| System | help, version, api version |
| Peer | list, detail, capabilities, statistics, set (add/save), delete, teardown, flush |
| Announce | route, flow, vpls, eor, operational |
| Withdraw | route, flow, vpls, watchdog |
| RIB | routes, best, status, clear |
| Log | levels, set (runtime log levels) |
| Metrics | values, list (Prometheus metrics) |
| Group | start, end (batching) |
| Monitor | monitor bgp (TUI dashboard), monitor event (live event streaming), monitor system netlink (kernel events) |
| Subscribe | request subscribe, request unsubscribe (event filtering) |
| OSPF | show ospf neighbor, interface, database, route, border-routers, spf |
| Reports | show warnings, show errors (cross-subsystem operational report bus) |
<!-- source: internal/component/plugin/server/command.go -- AllBuiltinRPCs -->
<!-- source: internal/plugins/ospf/register.go -- sdk.CommandDecl show ospf commands -->

### Dispatch keys are YANG paths

The daemon registers each built-in handler under its **YANG command path**, derived
from the tree (`LoadBuiltins`: `d.RegisterWithOptions(wireToPath[wireMethod], ...)`).
The path — not the wire method — is the dispatch key. Moving a `ze:command` container
in the YANG tree therefore renames the command and breaks anything that sends the old
path. Operator commands are safe to migrate; commands a plugin sends by their bare
path (over `dispatch-command` or an interactive plugin CLI session) are a wire break.
Before a verb-first migration of a noun-first built-in, grep for senders. See
`ai/rules/cli.md` "Migrating a Built-in Command's Path".
<!-- source: internal/component/plugin/server/command.go -- LoadBuiltins, IsReadOnlyPath -->

### A parent command never swallows a registered child

The daemon serves the LONGEST registered key that prefixes the input and hands the
rest to that handler as arguments. A parent registered at a short path would
therefore own the whole subtree below it, children another owner registered
included. `show bgp` is a builtin key; `show bgp rpki status` is a PLUGIN name that
`Dispatch` reaches only after the builtin match fails, so an unguarded match sends
the rpki subtree to the summary handler, which reads its first argument as an
address family.

`matchBuiltinTokens` refuses its match when a LONGER prefix of the input is itself a
registered command. It asks all three registries the dispatcher resolves from: the
builtin keys, the plugin registry, and the subsystem handlers. The test is a
registered PATH, never the presence of leftover tokens, because leftovers are how
every argument-taking command works: `show bgp ipv4` still reaches its
handler with the family.

The client-side lookup carries the same rule, and the two are separate because they
read different registries. Neither is derived from the other.
<!-- source: internal/component/plugin/server/command.go -- matchBuiltinTokens, longerCommandPath, isCommandPath -->
<!-- source: internal/component/command/registry/registry.go -- LookupLocal -->

### Offline fallback (read-only commands without a daemon)

Some read-only `show` commands must work with **no daemon reachable** — `show crashes`
(you inspect a crash precisely when the daemon has died) and `show host` (hardware
inventory before the daemon is up). Their owner registers an in-process handler with
`registry.RegisterOfflineFallback(path, handler)`. The CLI serves the command from the
daemon when it is reachable and calls the fallback **only after a connection failure**,
so the fallback never shadows the daemon. Because `cmdutil.RunCommand` rejects commands
absent from the CLI binary's tree before reaching the daemon path, it makes an exception
for paths that have a registered fallback, routing them through so the fallback is
reachable. Output is identical to the daemon RPC (both read the same detection library
/ crash files), so online and offline results match.
<!-- source: internal/component/command/registry/registry.go -- RegisterOfflineFallback, LookupOfflineFallback -->
<!-- source: cmd/ze/internal/cmdutil/cmdutil.go -- RunCommand offline-fallback routing -->
<!-- source: internal/component/cli/client/main.go -- runOfflineFallback (invoked on daemon-unreachable) -->

---

## Operational Report Bus (ze-show:warnings, ze-show:errors)

The report bus is a single in-process place for Ze subsystems to push
operator-visible issues. Any subsystem (BGP, config, interface, plugins)
can call `report.RaiseWarning` / `report.RaiseError` / `report.ClearWarning`,
and operators query the aggregate via two RPCs.

<!-- source: internal/core/report/report.go -- package godoc -->

### Severity contract

| Severity | Lifecycle | Storage | Cleared by |
|----------|-----------|---------|------------|
| `warning` | State-based. Condition is currently problematic. | Deduped map on `(Source, Code, Subject)`. Bounded by `warningCap` (default 1024, max 10000). Oldest-by-Updated evicted at cap. | `ClearWarning(source, code, subject)`, `ClearSource(source)`, or implicit eviction. |
| `error` | Event-based. Something already happened. | Ring buffer of `errorCap` events (default 256, max 10000). Oldest evicted on overflow. | Never; ring buffer aging only. |

Producers MUST pick the right severity. The bus does not auto-promote.
"Did anything actually fail or behave unexpectedly?" Yes -> error.
No, but it might soon -> warning.

<!-- source: internal/core/report/report.go -- Severity, RaiseWarning, RaiseError, ClearWarning, ClearSource -->

### Push API (for subsystems)

Subsystems import `internal/core/report` and call:

| Function | Purpose |
|----------|---------|
| `RaiseWarning(source, code, subject, message string, detail ...map[string]any)` | Add or refresh an active warning. Deduplicated. |
| `ClearWarning(source, code, subject string)` | Remove an active warning. No-op if missing. |
| `ClearSource(source string)` | Remove all active warnings for one source (shutdown cleanup). |
| `RaiseError(source, code, subject, message string, detail ...map[string]any)` | Append an error event to the ring buffer. No dedup. |
| `Warnings() []Issue` | Snapshot of all active warnings, most-recently-updated first. Consumed by ze-show:warnings handler. |
| `Errors(limit int) []Issue` | Recent N error events, newest first. limit 0 or negative returns all retained. Consumed by ze-show:errors handler. |

Empty or oversized fields (Source/Code > 64 bytes, Subject > 256, Message > 1024,
Detail > 16 keys) are rejected at the boundary with a debug log, protecting
the bus from buggy or malicious producers.

<!-- source: internal/core/report/report.go -- validFields, RaiseWarning, RaiseError, ClearWarning, ClearSource, Warnings, Errors -->

### Query RPCs

| WireMethod | Handler | Response shape |
|------------|---------|----------------|
| `ze-show:warnings` | `handleShowWarnings` in `internal/component/cmd/show/show.go` | `{"warnings": [Issue, ...], "count": N}` |
| `ze-show:errors` | `handleShowErrors` in `internal/component/cmd/show/show.go` | `{"errors": [Issue, ...], "count": N}` |
| `ze-show:traffic` | `handleShowTraffic` in `internal/component/traffic/cmd/traffic.go` | `{"interfaces": [...], "count": N}` or single interface detail |
| `ze-show:static` | `forwardShowStatic` in `internal/plugins/static/cmd_show.go` | JSON array of configured static routes (proxy to static plugin) |
| `ze-show:policy-routes` | `forwardShowPolicyRoutes` in `internal/plugins/policyroute/cmd_show.go` | JSON array of PBR policy routes (proxy to policyroute plugin) |
| `ze-show:policy-chain` | `handleShowPolicyChain` in `internal/component/bgp/plugins/cmd/policy/handler.go` | `{"chains": [{"peer": "...", "name": "...", "import": [{"name": "...", "canonical": "..."}], "export": [...]}]}` — per-peer effective filter chains, plain name plus canonical ref |
| `ze-show:policy-test` | `handleShowPolicyTest` in `internal/component/bgp/plugins/cmd/policy/handler.go` | `{"direction": "...", "peer": "...", "action": "accept\|reject\|modify", "trace": [PolicyTraceEntry], "text-before": "...", "text-after": "...", "changed-attrs": [...], "wire-changes": ["AS4_PATH suppressed", ...]}` — read-only policy dry-run, no forwarding or mutation |
| `ze-show:bmp-sessions` | `forwardShowBMPSessions` in `internal/component/bgp/plugins/bmp/cmd_show.go` | JSON array of BMP receiver sessions (proxy to BMP plugin) |
| `ze-show:bmp-peers` | `forwardShowBMPPeers` in `internal/component/bgp/plugins/bmp/cmd_show.go` | JSON array of BMP monitored peers (proxy to BMP plugin) |
| `ze-show:bmp-collectors` | `forwardShowBMPCollectors` in `internal/component/bgp/plugins/bmp/cmd_show.go` | JSON array of BMP sender collectors (proxy to BMP plugin) |
| `ze-show:bmp-rib` | `forwardShowBMPRib` in `internal/component/bgp/plugins/bmp/cmd_show.go` | BMP-monitored routes (proxy to BMP plugin, dispatches to RIB) |
| `ze-show:rr-status` | `forwardShowRRStatus` in `internal/component/bgp/plugins/rr/cmd_show.go` | `{"running": true}` (proxy to RR plugin) |
| `ze-show:rr-peers` | `forwardShowRRPeers` in `internal/component/bgp/plugins/rr/cmd_show.go` | JSON array of RR peer states (proxy to RR plugin) |
| `ze-show:system-sockets` | `handleShowSystemSockets` in `sockets_linux.go` | `{"sockets": [...], "count": N}` (Linux only) |
| `ze-show:system-kernel-log` | `handleShowSystemKernelLog` in `kernel_log_linux.go` | `{"entries": [...], "count": N}` (Linux only) |
| `ze-show:system-goroutines` | `handleShowSystemGoroutines` in `goroutines.go` | `{"total": N, "by-state": {...}, "mode": "..."}` |
| `ze-show:tcp-check` | `HandleTCPCheck` in `internal/plugins/diag/cmd/tcp_check.go` | `{"host": "...", "port": N, "result": "...", "latency-ms": N}` |
| `ze-show:traceroute` | `handleTraceroute` in `traceroute.go` | `{"target": "...", "hops": [{"hop": N, "addr": "...", "rtt-ms": N, "ttl": N}, ...]}` |
| `ze-show:capture-interface` | `handleCaptureInterface` in `capture_interface_linux.go` | pcap: `{"format": "pcap", "packets": N, "pcap": "base64...", "snap-len": N}`; text: `{"format": "text", "packets": N, "lines": [...]}` (Linux only) |
| `ze-show:system-file-descriptors` | `handleShowSystemFD` in `fd_linux.go` | `{"total": N, "by-type": {...}, "soft-limit": N, "hard-limit": N}` (Linux only) |
| `ze-show:dns-lookup` | `handleDNSLookup` in `internal/component/resolve/cmd/show_dns.go` | `{"name": "...", "type": "...", "records": [...], "query-time-ms": N}` |
| `ze-show:dns-cache-stats` | `handleDNSCacheStats` in `internal/component/resolve/cmd/show_dns.go` | `{"entries": N, "capacity": N, "hits": N, "misses": N, "hit-rate": N, "miss-rate": N, "evictions": N, "expired": N}` |
| `ze-show:dns-cache-list` | `handleDNSCacheList` in `internal/component/resolve/cmd/show_dns.go` | `{"entries": [...], "count": N}` |
| `ze-show:dns-cache-record` | `handleDNSCacheRecord` in `internal/component/resolve/cmd/show_dns.go` | `{"entries": [...], "count": N, "filter": "name"}` |
| `ze-clear:dns-cache` | `handleClearDNSCache` in `internal/component/resolve/cmd/dns.go` | `{"action": "clear-all"}` |
| `ze-clear:dns-cache-stats` | `handleClearDNSCacheStats` in `internal/component/resolve/cmd/dns.go` | `{"action": "reset-stats"}` |
| `ze-clear:dns-cache-record` | `handleClearDNSCacheRecord` in `internal/component/resolve/cmd/dns.go` | `{"action": "delete-entry", "name": "...", "removed": N}` or `{"action": "delete-entry", "name": "...", "type": "...", "found": bool}` |
| `ze-show:system-profile` | `handleShowSystemProfile` in `profile.go` | `{"type": "...", "format": "pprof-base64", "data": "..."}` |
| `ze-show:system-memory-map` | `handleShowSystemMemoryMap` in `memory_map_linux.go` | `{"vm-rss-kb": N, "vm-size-kb": N, ...}` (Linux only) |
| `ze-show:system-update` | `handleShowSystemUpdate` in `internal/plugins/update-cmd/cmd/show.go` | `{"backend": "ze-self-update"\|"gokrazy-ab", "running-version": "...", "remote-version": "...", "update-available": bool, "status": "...", "download-status": "...", "staged-version": "...", "gokrazy-reachable": bool, "gokrazy-features": [...]}` |
| `ze-show:system-update-history` | `handleShowSystemUpdateHistory` in `internal/plugins/update-cmd/cmd/show.go` | `{"history": [{"timestamp": "...", "from": "...", "to": "...", "result": "..."}], "count": N}` |
| `ze-update:system-firmware-check` | `handleFirmwareCheck` in `firmware.go` | `{"running-version": "...", "update-available": bool, ...}` or on gokrazy `{"backend":"gokrazy-ab", "status":"unsupported", "message":"updates managed by gokrazy"}` |
| `ze-update:system-firmware-download` | `handleFirmwareDownload` in `firmware.go` | `{"downloaded-version": "...", "status": "complete"}` |
| `ze-update:system-firmware-apply` | `handleFirmwareApply` in `firmware.go` | `{"applied-version": "...", "status": "restarting"}` |
| `ze-update:system-firmware-restart` | `handleFirmwareRestart` in `firmware.go` | `{"status": "restarting"}` |
| `ze-update:system-firmware-rollback` | `handleFirmwareRollback` in `firmware.go` | `{"status": "rolling back"}` |
| `ze-show:interface` (no args) | `handleShowInterface` in `internal/component/iface/cmd/show_interface.go` | JSON array of `InterfaceInfo`. A stray token is refused with the usage text: every subcommand below has its own wire method |
| `ze-show:interface-brief` | `handleShowInterfaceBrief` in `internal/component/iface/cmd/show_interface.go` | `{"interfaces": [{name, state, mtu, address?}], "count": N}` |
| `ze-show:interface-type` (args: `<type>`) | `handleShowInterfaceType` in `internal/component/iface/cmd/show_interface.go` | `{"interfaces": [InterfaceInfo]}` for that type. An unmatched type is refused, and the refusal lists the types the running set has |
| `ze-show:interface-errors` | `handleShowInterfaceErrors` in `internal/component/iface/cmd/show_interface.go` | `{"interfaces": [{name, rx-errors, rx-dropped, tx-errors, tx-dropped}]}`, only the links with a non-zero counter |
| `ze-show:interface-rate` (args: `[<name>]`) | `handleShowInterfaceRateCmd` in `internal/component/iface/cmd/show_interface.go` | JSON array of `InterfaceRate` (all) or single object (named); fields: `name`, `rx-bps`, `tx-bps`, `rx-pps`, `tx-pps`, `stats` |
| `ze-monitor:interface-rate` | `streamInterfaceRate` in `internal/component/iface/cmd/interface_rate.go` | Streaming JSON lines (1/s); optional `<name>` filter |
| `ze-show:storage-smart` | `handleShowStorageSmart` in `internal/component/storage/show.go` | JSON array of per-device objects: `name`, `transport`, `healthy`, `temp-celsius`, `power-on-hours`, `error-count`, `percent-used` (NVMe), `available-spare` (NVMe), `smart-enabled`, `last-checked`, `last-short-test`, `last-long-test`. Returns error if SMART management not configured. |
| `ze-show:flow-export` (args: `[<collector>]`) | `handleShowFlowExport` in `internal/plugins/flowexport/cmd_show.go` | No arg: JSON array of per-collector objects. Named: single collector object, or error `collector not found: <name>`. Per-collector fields: `name`, `address`, `port`, `protocol`, `datagrams-sent`, `bytes-sent`, `errors`, `sequence`, `last-export-time` (Unix seconds, omitted before first poll). When unconfigured: `{"status": "not-configured"}`. Backed by `flowexport.Exporter.Status()`. |
| `ze-show:traffic-stat` (args: `[name <interface>]`) | `handleShowTraffic` in `internal/component/trafficstat/cmd/traffic.go` | One-shot aggregated snapshot: `{"at": "RFC3339", "severity": "normal\|caution\|danger", "degraded": bool, "interfaces": [{name, rx-bps, tx-bps, rx-pps, tx-pps}], "top-source-ips": [{address, bps}], "top-dest-ips": [{address, bps}], "top-ports": [{port, service, proto, bps, amplification?}], "protocol-mix": [{proto, name, bps, percent}], "history": [float64]}`. Optional `name <interface>` filters the interfaces array. When no collector data: `"degraded": true` with interface rates only. |
| `ze-monitor:traffic-stat` (args: `[name <interface>]`) | `streamTraffic` in `internal/component/trafficstat/cmd/traffic.go` | Streaming JSON lines (1/s) with the same shape as `ze-show:traffic-stat`. Attaches as a consumer on connect, detaches on disconnect (lazy lifecycle). Also registered as a `MonitorProvider` for full-screen TUI rendering via `createTrafficMonitorSession` in `cmd/render.go`. |
| `ze-show:traffic-feature` (args: `[name <address>]`) | `handleShowTrafficFeature` in `internal/component/trafficfeature/cmd/traffic_feature.go` | Neutral per-source feature snapshot: `{"degraded": bool, "top-source-ips": [{address, fan-out, out-in-ratio, port-entropy, new-peer, rare-port, beaconing}]}`. `out-in-ratio` is the string `"inf"` when a source has no inbound bytes (else a float). Optional `name <address>` filters to one source. Facts only (no verdict); the anomaly detection family applies judgment. |
| `ze-show:anomaly` (no args) | `handleShowAnomaly` in `internal/plugins/anomaly/detect/show.go` | Recent behavioral anomaly incidents (report-only): `{"enabled": bool, "incidents": [{entity, cohort, score, severity, at, fired-features: [{name, z}]}]}`. Bounded recent-incident ring; empty until an entity's correlated deviation confirms. `enabled` is false when the detector is not running. The `anomaly/shape` responder consumes the underlying `anomaly-detect` events; this command is the read-only view. |
| `ze-show:anomaly-observe` (no args) | `handleShowAnomalyObserve` in `internal/plugins/anomaly/observe/show.go` | Behavioral anomaly incident LIFECYCLE, newest first: `{"enabled": bool, "active-count": N, "incidents": [{id, interface, entity, cohort, fired-features: [{name, z}], score, severity, start-time, end-time, active}]}`. `end-time` is omitted while `active` is true. It is set when the incident clears, or when the stale timeout finalizes it. A finished incident's duration is readable here and nowhere else. `enabled` is false when the plugin is not running. |
| `ze-show:anomaly-shape` (no args) | `handleShowAnomalyShape` in `internal/plugins/anomaly/shape/show.go` | Shadow-first responder status: `{"enabled": bool, "mode": "shadow"\|"armed", "action": "limit"\|"drop", "kill-switch": bool, "armed-count": N, "armed": [source, ...]}`. `enabled` is false before configuration. In shadow mode (default) nothing is installed; armed sources carry a live per-source firewall action with a timed auto-revert. |
| `ze-show:traffic-usage` (args: `[name <interface>]`) | `handleShowTrafficUsage` in `internal/plugins/trafficusage/show.go` | No arg: JSON array of per-interface objects. `name <interface>`: single interface object. Per-interface fields: `ingress-ports`, `egress-ports`, `map-entries`, and (only when `track-ip` is enabled) `ingress-ips`, `egress-ips`. Bad args: error `usage: show traffic usage [name <interface>]`. When unconfigured: `{"status": "not-configured"}`. |
| `ze-show:pki-certificates` | `handleShowPKICertificates` in `internal/component/pki/show.go` | `{"certificates": [CertSummary, ...], "count": N}` |
| `ze-show:pki-certificate` (args: `<name> [pem \| bundle pem \| fingerprint [algo]]`) | `handleShowPKICertificate` in `internal/component/pki/show.go` | No sub-command: full detail map. `pem`: `{"pem": "..."}`. `bundle pem`: `{"pem": "cert+key"}`. `fingerprint`: `{"name": "...", "algorithm": "sha256", "fingerprint": "aa:bb:..."}`. |

Both warnings/errors handlers accept optional `source <name>` filter and
errors accepts `count <N>` limit. Return a non-nil empty slice when empty.

<!-- source: internal/component/cmd/show/show.go -- handleShowWarnings, handleShowErrors -->
<!-- source: internal/plugins/static/cmd_show.go -- ForwardToPlugin proxy -->
<!-- source: internal/plugins/policyroute/cmd_show.go -- ForwardToPlugin proxy -->
<!-- source: internal/component/bgp/plugins/bmp/cmd_show.go -- ForwardToPlugin proxy -->
<!-- source: internal/component/bgp/plugins/rr/cmd_show.go -- ForwardToPlugin proxy -->
<!-- source: internal/component/cmd/show/yang/ze-cli-show-cmd.yang -- top-level warnings / errors containers -->

### Issue JSON shape

| Field | JSON type | Notes |
|-------|-----------|-------|
| `source` | string | Subsystem name (lowercase, short: `bgp`, `config`, `iface`, ...) |
| `code` | string | Kebab-case identifier (`prefix-threshold`, `notification-sent`, ...) |
| `severity` | string | `"warning"` or `"error"` (MarshalJSON converts the internal uint8 enum to the label) |
| `subject` | string | What the issue is about (peer address, transaction id, file path) |
| `message` | string | Human-readable one-liner |
| `detail` | object | Optional. Omitted via `omitempty` when nil or empty. Keys and values are producer-defined. |
| `raised` | RFC 3339 time | First appearance |
| `updated` | RFC 3339 time | Most recent raise (warnings advance; errors equal raised) |

<!-- source: internal/core/report/report.go -- Issue struct with json tags, Severity.MarshalJSON -->

### Day-one BGP producers

| Severity | Code | Subject | Detail keys | Source function |
|----------|------|---------|-------------|-----------------|
| warning | `prefix-threshold` | `<peerAddr>/<afi>/<safi>` | `family`, `count`, `warning`, `maximum` | `raisePrefixThreshold` in `reactor/session_prefix.go`, called from `applyPrefixCheck` |
| warning | `prefix-stale` | peer address | `updated` | `RaisePrefixStale` in `reactor/session_prefix.go`, called from `reactor_peers.go` at peer add (and clear at remove) |
| error | `notification-sent` | peer address | `code`, `subcode`, `direction=sent` | `raiseNotificationError("sent", ...)` in `reactor/session_prefix.go`, called from `Peer.IncrNotificationSent` |
| error | `notification-received` | peer address | `code`, `subcode`, `direction=received` | `raiseNotificationError("received", ...)` in `reactor/session_prefix.go`, called from `Peer.IncrNotificationReceived` |
| error | `session-dropped` | peer address | `reason` | `raiseSessionDropped` in `reactor/session_prefix.go`, called from `peer_run.go` FSM Established->Idle branch when no notification was exchanged |

The `session-dropped` producer is suppressed when a NOTIFICATION was sent
or received during the same session lifecycle: the operator already sees
that event in `show errors` so a duplicate session-dropped would be noise.
Tracking is via `Peer.notificationExchanged atomic.Bool`, reset at the
start of each `runOnce` iteration.

<!-- source: internal/component/bgp/reactor/session_prefix.go -- reportCode* constants, raise* helpers -->
<!-- source: internal/component/bgp/reactor/peer.go -- Peer.notificationExchanged -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- FSM Established->Idle branch + runOnce reset -->

### Login banner integration

The Ze CLI login banner reads from the same report bus (filtered by
source `bgp`). One active warning displays the detail line; multiple
warnings collapse to a count line pointing at `show warnings`. This
is the single source of truth for "what's wrong right now" across
the connect path and the show command.

<!-- source: internal/component/bgp/config/loader.go -- collectPrefixWarnings -->

### Capacity limits and env vars

| Env var | Default | Maximum | Registered by |
|---------|---------|---------|---------------|
| `ze.report.warnings.max` | 1024 | 10000 | `env.MustRegister` in `internal/core/report/report.go` |
| `ze.report.errors.max` | 256 | 10000 | same |

Operator values exceeding the maximum are clamped and logged at warn level.
Zero or negative values fall back to the default. This prevents an env-var
typo from causing memory exhaustion.

<!-- source: internal/core/report/report.go -- newStore, maxWarningCap, maxErrorCap -->

---

## Target-First Syntax

### Process Lifecycle Commands

```
request shutdown         # Graceful shutdown
request reboot           # Graceful shutdown + OS reboot (requires root on Linux)
request reload           # Reload configuration
request halt             # Goroutine dump + immediate exit
show status              # Get process status
```

### Session Commands

```
plugin session ready     # Signal plugin init complete
plugin session ping      # Health check
plugin session bye       # Disconnect
```
<!-- source: internal/core/ipc/yang/ze-plugin-api.yang -- session RPCs -->

### BGP Plugin Configuration

```
bgp plugin encoding json     # Set event encoding to JSON (default)
bgp plugin encoding text     # Set event encoding to human-readable text
bgp plugin format hex        # Wire bytes as hex string
bgp plugin format base64     # Wire bytes as base64
bgp plugin format parsed     # Decoded fields only (default)
bgp plugin format full       # Both parsed AND wire bytes
bgp plugin ack sync          # Wait for wire transmission
bgp plugin ack async         # Return immediately (default)
```
<!-- source: internal/component/bgp/yang/ze-bgp-api.yang -- plugin-encoding, plugin-format, plugin-ack RPCs -->

### Event Subscription Commands

A plugin declares what it CAN handle. The peer's configuration decides what it
GETS. A peer-scoped event is delivered to a process when both halves name it:
the process subscribed to the event, and the peer's `attach process <name>`
block grants that type in that direction. A peer that attaches no block for a
process feeds it nothing.

```
request subscribe <namespace> event <type> [direction received|sent|both]
request subscribe peer <selector> event <type> [direction ...]
request subscribe plugin <name> <namespace> event <type> [direction ...]
request unsubscribe <namespace> event <type> [direction received|sent|both]
```

**Precedence.** The configuration is durable receive authorization and is
rebuilt on every config apply. A `request subscribe` typed at a running daemon
is a live capability override: it can add a type the process did not declare at
startup only where the peer's block grants that type. It cannot widen the
configured grant, and the next config apply discards it.
<!-- source: internal/component/plugin/server/delivery_graph.go -- (*Server).PeerScopedProcs, (*Server).DiscardRuntimeSubscriptions -->

At plugin ready, and again after every apply, ze reports each peer, process and
event type the two halves disagree about. Two lines an operator sees:

```
event delivery: the config grants an event the plugin never declared  peer=... process=... granted=update-received
event delivery: the plugin declared an event the peer does not grant it  peer=... process=... declared=state
```

**Namespaces:**
- `bgp` - BGP protocol events
- `rib` - RIB events (cache, route changes)

**BGP event types:**

| Event | Has Direction | Description |
|-------|---------------|-------------|
| `update` | ✅ | UPDATE message |
| `open` | ✅ | OPEN message |
| `notification` | ✅ | NOTIFICATION message |
| `keepalive` | ✅ | KEEPALIVE message |
| `refresh` | ✅ | ROUTE-REFRESH message |
| `state` | ❌ | Peer state change (up/down) |
| `negotiated` | ❌ | Capability negotiation complete |
| `rpki` | ❌ | RPKI validation result (from bgp-rpki plugin) |
| `update-rpki` | ✅ | UPDATE merged with RPKI validation (from bgp-rpki-decorator) |

Plugins may register additional event types via `Registration.EventTypes`. These are validated at runtime against the dynamic registry.
<!-- source: internal/component/plugin/registry/registry.go -- Registration.EventTypes -->

**RIB event types:**

| Event | Description |
|-------|-------------|
| `cache` | Cache entry events |
| `route` | Route change events |

**Examples:**
```
request subscribe bgp event update                              # All peers, both directions
request subscribe bgp event update direction received           # Received only
request subscribe peer upstream1 event update                    # Specific peer
request subscribe peer * event state                            # All peers, state changes
request subscribe peer !upstream1 event update direction sent      # Exclude one peer
request subscribe rib event route                               # RIB route events
```

### Monitor Commands

Stream live events or display live dashboards. All monitor commands follow verb-first syntax: `monitor <module> [args...]`.

```
monitor event                                                # All events, all peers
monitor event peer <addr>                                    # Filter by peer address
monitor event peer *                                         # Explicit all peers
monitor event include <type>,<type>                          # Only listed event types
monitor event exclude <type>,<type>                          # All types except listed
monitor event direction received                             # Received events only
monitor event direction sent                                 # Sent events only
monitor event include update peer <addr> direction received  # Combined filters
monitor bgp                                                  # Live peer dashboard (TUI)
```

| Keyword | Values | Default |
|---------|--------|---------|
| `peer` | IP address, peer name, `!exclusion`, or `*` | `*` (all peers) |
| `include` | Comma-separated event types | All types (mutually exclusive with `exclude`) |
| `exclude` | Comma-separated event types | None (mutually exclusive with `include`) |
| `direction` | `received`, `sent` | Both directions |

Keywords may appear in any order. `include` and `exclude` are mutually exclusive.

Event types span all namespaces: BGP (update, open, notification, keepalive, refresh, state, negotiated, eor, congested, resumed, rpki) and RIB (cache, route). Types are validated at parse time.
<!-- source: internal/component/plugin/server/event_monitor.go -- ParseEventMonitorArgs -->

Wire method: `ze-event:monitor`. Supports pipe operators: `| json`, `| table`, `| match`.
<!-- source: internal/component/plugin/server/monitor.go -- MonitorManager -->

**Note:** `monitor bgp` is the live peer dashboard (TUI only). `monitor event` streams live events (SSH exec or TUI). `monitor system netlink` streams kernel netlink events (SSH exec or TUI, Linux only).

#### Netlink Monitor

Stream kernel route, link, and address change events as one JSON line per event. Replaces `ip monitor` on gokrazy appliances.

```
monitor system netlink                   # All netlink groups (route + link + address)
monitor system netlink route             # Route changes only
monitor system netlink link              # Link state changes only
monitor system netlink address           # Address changes only
monitor system netlink all               # Explicit all (same as no argument)
```

Wire method: `ze-monitor:system-netlink`. Linux only; returns "not available on this platform" on other OSes.
<!-- source: internal/component/iface/cmd/monitor_netlink_linux.go -- streamNetlinkMonitor -->
<!-- source: internal/component/cli/model_dashboard.go -- isDashboardCommand -->

### System Commands

```
system help              # Show help (uses dispatcher, includes plugin commands)
system version software  # Show Ze version
system version api       # Show IPC protocol version
system subsystem list    # List available subsystems
system command list      # List all commands (builtin + plugin)
system command list verbose  # List with source (builtin/process name)
system command help "<name>" # Show command details
system command complete "<partial>"  # Complete command names
system command complete "<cmd>" args [<completed>...] "<partial>"  # Arg completion
```
<!-- source: internal/core/ipc/yang/ze-system-api.yang -- system RPCs -->

### Process Lifecycle Commands

```
request shutdown         # Gracefully shutdown
request reboot           # Gracefully shutdown then reboot the system
show status              # Show process status
request reload           # Reload the configuration
show reload-status       # Show how many config reloads have been processed
```

#### show reload-status (the reload fence)

Returns the reload generation counter as JSON:

```json
{"generation": 3, "last-outcome": "applied", "last-reload-at": "2026-07-16T09:12:44Z"}
```

| Field | Meaning |
|-------|---------|
| `generation` | Reloads PROCESSED since daemon start. Starts at 0. |
| `last-outcome` | `applied`, `failed`, or `none` before the first reload. |
| `last-reload-at` | RFC3339 UTC completion time; empty string before the first reload. |

`generation` advances on **every** processed reload, including one that rejected
a change or changed nothing. That is what makes it a fence rather than a
statistic: a reload that rejects a change (l2tp refusing a listener rebind, for
example) leaves no other observable trace, so an observer wanting to assert "the
reload ran and correctly left this alone" has nothing else to wait on. It reads
`generation`, triggers the reload, polls until the value advances, then asserts.
Waiting on a timer instead makes the assertion pass vacuously against a reload
that had not started.

A reload refused with `ErrReloadInProgress` does not advance the counter: it was
queued, not processed, and the replay advances it.

The counter is observational only; nothing reads it to make a decision, so it
cannot change what a reload accepts or rejects.

<!-- source: internal/component/plugin/server/reload_generation.go -- counter state, outcome vocabulary -->
<!-- source: cmd/ze/hub/main_reload.go -- doReload, the increment site, after engine.Reload -->
<!-- source: internal/component/cmd/show/reload_status.go -- ze-show:reload-status handler + JSON shape -->

### Peer Commands

```
show bgp peer list                    # List all peers
show bgp peer <selector> detail       # Show specific peer detail
show bgp peer <selector> capabilities # Show specific peer capabilities
show bgp peer <selector> statistics   # Show specific peer statistics
show bgp peer <selector> history      # Show FSM transition history
request peer <selector> teardown [<cease-subcode>]  # Disconnect peer
delete bgp peer <name>             # Remove dynamic peer
request peer <sel> flush           # Wait for forward pool to drain (barrier)
```
<!-- source: internal/component/bgp/yang/ze-bgp-api.yang -- peer RPCs -->

### Cache Commands (Ze)

> **Design history:** summary 148 was retired on 2026-08-01 and was not carried
> into `plan/learned/DESIGN-HISTORY.md`. That file's header gives the
> git-recovery route. What survives of the cache pattern is in its "BGP engine:
> wire encoding and RIB" > Load-bearing invariants: cache `Ack` and `Retain`
> are independent refcount axes, and engine cache ack is cumulative.

```
request cache forward <id> <sel>    # Forward cached UPDATE to peers
request cache retain <id>           # Prevent eviction
request cache release <id>          # Allow eviction (reset TTL)
request cache expire <id>           # Remove immediately
show cache                          # List cached message IDs

# Batch variants (comma-separated IDs, max 1000):
request cache forward <id1>,<id2>,...,<idN> <sel>  # Batch forward
request cache release <id1>,<id2>,...,<idN>        # Batch release
```

The cache commands enable route reflection via API:
1. Received UPDATEs are assigned a unique msg-id (per-UPDATE, not per-NLRI)
2. API outputs UPDATE info with msg-id
3. External process decides routing
4. The `request cache forward` command references msg-id (zero-copy when contexts match)
5. Cache entries expire after configurable TTL (default 60s) unless retained
<!-- source: internal/component/bgp/reactor/reactor.go -- cache forward -->

#### Fast-path typed SDK (rs-fastpath-3)

The text-RPC `request cache forward <id> <sel>` path tokenises, parses, and walks the command registry on every call. Plugins that forward many cached UPDATEs per second (route server, future route reflector) use a typed SDK pair instead:

```go
Plugin.ForwardCached(ctx, ids []uint64, destinations []netip.AddrPort) error
Plugin.ReleaseCached(ctx, ids []uint64) error
```

Both methods round-trip via `DirectBridge` when the plugin is in-process (zero socket I/O, no tokenisation, no command-registry lookup) and fall back to the `ze-plugin-engine:forward-cached` / `:release-cached` methods over the newline-framed plugin RPC connection when the plugin is out-of-process. The engine handler is `reactorAPIAdapter.ForwardUpdatesDirect` / `ReleaseUpdates`.

Destinations are peer addresses (`netip.AddrPort`). Port-0 entries match any peer instance with the same address. Cap: 4096 destinations per call (override via `ze.fwd.dest.cap`); exceeding the cap is an explicit error, not silent truncation. Empty destination list returns an error too, guarding against an accidental wildcard broadcast — use `ReleaseCached` to ack without forwarding.

Per-source ordering, egress filter chains, AS-PATH prepend, next-hop policy, replay-on-new-peer, and every other forwarding invariant are preserved. Only the transport differs from the text-RPC path.
<!-- source: pkg/plugin/sdk/sdk_engine.go -- Plugin.ForwardCached, Plugin.ReleaseCached -->
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge.ForwardCached, SetForwardCached -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward_batch.go -- ForwardUpdatesDirect, ReleaseUpdates, maxForwardDestinations -->

### Log Commands (Ze)

```
show log levels                       # Show all subsystem log levels (JSON map)
request log level <logger> <level>    # Change subsystem log level at runtime
```

Levels: `debug`, `info`, `warn`, `err`. Changes take effect immediately via `slog.LevelVar` atomic swap. Only loggers created via `slogutil.Logger()` or `slogutil.LazyLogger()` (non-disabled) are shown and modifiable.
<!-- source: internal/plugins/log/yang/ze-log-cmd.yang -- log command YANG -->

### Metrics Commands (Ze)

```
show metrics values       # Dump Prometheus text format output
show metrics list         # List metric names only (no values)
show metrics pool         # Per-attribute pool occupancy and dedup rates
```

Requires telemetry to be enabled in config (`telemetry { prometheus { ... } }`). Returns error if metrics registry is not available.

`show metrics pool` returns 13 per-attribute pools (Origin, AS-Path, LocalPref, MED, NextHop,
Communities, LargeCommunities, ExtCommunities, ClusterList, OriginatorID, AtomicAggregate,
Aggregator, OtherAttrs) with live/dead slots, bytes, intern count, dedup hit rate.

<!-- source: internal/component/cmd/metrics/yang/ -- ze-bgp-cmd-metrics-api.yang -->

### Peer Selectors

```
peer *                   # All peers
peer upstream1            # Specific peer by name
peer !upstream1           # All peers EXCEPT this one (for route reflection)
```
<!-- source: internal/core/selector/selector.go -- Selector -->

The `!<ip>` negated selector is useful for route reflection:
```
# Forward update to all peers except the source
request cache forward 12345 !upstream1
```

> **Note:** Filter selectors (`[local-as ...]`, `[peer-as ...]`) from ExaBGP multi-session
> draft are not supported — the draft never became an RFC.

### Route Commands (update text)

All route operations use unified `update text` syntax with flat attribute declarations
(no `set` keyword) and keyword aliases (short forms accepted, see Keyword Aliases below):
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- ParseUpdateText -->

```bash
# Announce routes (flat attributes, no 'set')
peer <selector> update text next <ip> [attributes...] nlri <family> add prefix <prefix>...

# Withdraw routes
peer <selector> update text nlri <family> del prefix <prefix>...

# End-of-RIB marker (RFC 4724)
peer <selector> update text nlri <family> eor

# VPLS (L2VPN/VPLS)
peer <selector> update text nlri l2vpn/vpls add rd <rd> ve-id <n> ve-block-offset <n> ve-block-size <n> label-base <n>

# EVPN (L2VPN/EVPN)
peer <selector> update text nlri l2vpn/evpn add mac-ip rd <rd> mac <mac> [ip <ip>] label <n>
peer <selector> update text nlri l2vpn/evpn add ip-prefix rd <rd> prefix <prefix> label <n>
peer <selector> update text nlri l2vpn/evpn add multicast rd <rd> ip <ip>
```

### Route Commands (update cursor)

Stateful cursor mode for efficient replay of stored routes on reconnect. The engine
maintains attribute state per (plugin, peer) pair. Subsequent commands send only
changed attributes (delta encoding), reducing per-call overhead.
<!-- source: internal/component/bgp/plugins/cmd/update/cursor.go -- handleUpdateCursor -->

```bash
# First command: establish full attribute state + announce NLRIs
peer <selector> update cursor origin igp as-path [65001] med 100 \
  next-hop 10.0.0.1 nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24

# Delta: only changed attributes, rest inherited from cursor
peer <selector> update cursor as-path [65001 65003] \
  nlri ipv4/unicast add 10.1.0.0/24

# NLRIs only (all attributes inherited)
peer <selector> update cursor nlri ipv4/unicast add 10.2.0.0/24

# Remove an attribute from cursor
peer <selector> update cursor del med nlri ipv4/unicast add 10.3.0.0/24

# Clear cursor state (call after replay completes)
peer <selector> update cursor done
```

Cursor mode supports announce-only (`nlri <family> add`). Withdrawals are not
supported because replay re-sends stored routes, never withdrawals.

### Removed Commands (use update text instead)

The following legacy commands have been removed:

| Old Command | Replacement |
|-------------|-------------|
| `announce ipv4/unicast <p> next-hop <nh>` | `update text next <nh> nlri ipv4/unicast add prefix <p>` |
| `announce ipv6/unicast <p> next-hop <nh>` | `update text next <nh> nlri ipv6/unicast add prefix <p>` |
| `announce eor <afi> <safi>` | `update text nlri <family> eor` |
| `announce vpls ...` | `update text nlri l2vpn/vpls add ...` |
| `announce l2vpn ...` | `update text nlri l2vpn/evpn add ...` |
| `withdraw ipv4/unicast <p>` | `update text nlri ipv4/unicast del prefix <p>` |
| `withdraw ipv6/unicast <p>` | `update text nlri ipv6/unicast del prefix <p>` |
| `withdraw vpls ...` | `update text nlri l2vpn/vpls del ...` |
| `withdraw l2vpn ...` | `update text nlri l2vpn/evpn del ...` |

### Watchdog Commands

```
request bgp watchdog announce <name> [med <N>] [peer]  # Send all routes in pool (optional MED override)
request bgp watchdog withdraw <name> [peer]            # Withdraw all routes in pool from peers
```

Routes are tagged with a pool when announced:
```bash
update text nhop set 10.0.0.1 nlri ipv4/unicast add prefix 1.0.0.0/24 watchdog set mypool
```

> **Note:** `watchdog set` in wire-mode updates is parsed but not yet
> implemented. The `request bgp watchdog announce`/`request bgp watchdog withdraw`
> pool commands work independently of this tagging.

### RIB Commands

```
show bgp rib [filters...] [terminal]        # Unified route display with pipeline
    source filters: received | advertised
    filters: peer <selector>, path <pattern>, prefix <pattern>, community <value>,
             family <afi/safi>
    terminals: count, histogram, graph (AS topology box-drawing)
show bgp rib best [filters...] [terminal]   # Best-path per prefix (RFC 4271 §9.1.2)
show bgp rib status                         # RIB status (peer/route counts)
clear bgp rib in <selector>                  # Clear Adj-RIB-In (* for all peers)
clear bgp rib out <selector> [family]        # Resend Adj-RIB-Out (* for all, optional family)
request bgp rib inject <peer> <family> <prefix> [attrs] # Insert route into Adj-RIB-In (no session needed)
request bgp rib withdraw <peer> <family> <prefix>       # Remove route from Adj-RIB-In
show bgp rib rpf <family> <source-addr>      # RPF lookup (longest-prefix-match in Loc-RIB)
```

Generic pipes such as `match`, `json`, `ndjson`, `table`, `text`, `yaml`, `raw`, `resolve`, `origin`, `log`, `no-more`, `display`, and `fill` apply to the answer the command produced. The DAEMON runs them, on every surface. `execMiddleware` splits the chain off an SSH exec command and applies it. `ze cli` sends the chain intact and prints what comes back. Only the daemon holds the configuration, so only the daemon can honor `environment cli format default`.
<!-- source: internal/component/ssh/ssh.go -- execMiddleware -->
<!-- source: internal/component/cli/client/main.go -- Execute, commandWithFormat -->
<!-- source: internal/component/cli/client/answer.go -- daemonOutput, newDaemonOutput -->

RIB route filters such as `received`, `advertised`, `peer`, `family`, `prefix`, `path`, and `community` are command-specific filters registered by the RIB command and folded into the RIB iterator request before route output is generated.

Inject attributes: `origin <igp|egp|incomplete>`, `nhop|nexthop <ip>`, `aspath <asn,asn,...>`, `localpref <n>`, `med <n>`. Peer address is a label (valid IP, no session required). Only simple prefix families (IPv4/IPv6 unicast/multicast). IPv4-mapped IPv6 next-hops accepted.
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- injectRoute, withdrawRoute -->
<!-- source: internal/component/bgp/plugins/rib/yang/ze-rib-api.yang -- RIB RPCs -->

#### Inter-Plugin RIB Commands (GR/LLGR)

These commands are dispatched between plugins (bgp-gr to bgp-rib) and are not intended for direct user invocation:

```
request bgp rib retain-routes <peer>                    # Retain routes for peer (GR activation)
request bgp rib release-routes <peer>                   # Release retained routes
request bgp rib mark-stale <peer> <restart-time> [level]  # Mark routes stale (level: 1=GR, 2=LLGR)
request bgp rib purge-stale <peer> [family]             # Purge stale routes (optionally per-family)
request bgp rib attach-community <peer> <family> <hex>  # Attach community to stale routes in family
request bgp rib delete-with-community <peer> <family> <hex>  # Delete routes carrying community in family
```

### Group Commands (Batching)

```
group start [attributes ...]     # Start batch with shared attributes
peer <selector> update text ...
peer <selector> update text ...
group end                         # End batch, send all
```
<!-- source: internal/component/bgp/transaction/commit_manager.go -- CommitManager -->

---

## Action-First Syntax (Legacy)

### Show Commands

```
show neighbor [summary|extensive|configuration]
show adj-rib in [<afi> <safi>]
show adj-rib out [<afi> <safi>]
```

### Announce/Withdraw

All route operations now use `update text` syntax:

```bash
update text next <ip> [attributes...] nlri <family> add prefix <prefix>
update text nlri <family> del prefix <prefix>
update text nlri <family> eor
```

> **Note:** Legacy `announce`/`withdraw` commands have been removed.
> See "Removed Commands" section above for migration table.

### Control

```
teardown <peer-ip> <code> [<reason>]
shutdown
reload
restart
reset
enable-ack
disable-ack
silence-ack
help
version
```

---

## API Content Configuration (Ze)

### Attribute Filtering

`ContentConfig.Attributes` limits which path attributes are parsed for API
output. It is set by the engine, not by a peer's attach block: the
`content { encoding format attribute }` container inside `attach process`
parses and reaches no field of `ProcessBinding`, so a peer cannot ask for one
rendering while another peer asks for a different one.
<!-- source: internal/component/bgp/reactor/peer_settings.go -- ProcessBinding -->

Available attribute names:
| Name | Code | Description |
|------|------|-------------|
| `origin` | 1 | ORIGIN |
| `as-path` | 2 | AS_PATH |
| `next-hop` | 3 | NEXT_HOP |
| `med` | 4 | MULTI_EXIT_DISC |
| `local-pref` | 5 | LOCAL_PREF |
| `atomic-aggregate` | 6 | ATOMIC_AGGREGATE |
| `aggregator` | 7 | AGGREGATOR |
| `community` | 8 | COMMUNITIES |
| `originator-id` | 9 | ORIGINATOR_ID |
| `cluster-list` | 10 | CLUSTER_LIST |
| `extended-community` | 16 | EXTENDED_COMMUNITIES |
| `large-community` | 32 | LARGE_COMMUNITIES |
| `all` | - | All attributes (default) |
<!-- source: internal/component/bgp/types/contentconfig.go -- ContentConfig -->

Benefits of partial parsing:
- Reduced CPU (only parse what's needed for routing decision)
- Reduced memory (don't store full parsed attributes)
- Wire bytes preserved for zero-copy forwarding

### NLRI Family Filtering

`ContentConfig.NLRI` limits which address families are included in API output.
Like the attribute filter, it is engine-set: no config leaf reaches it.

Available families:
| Config Syntax | Canonical Name |
|---------------|----------------|
| `ipv4/unicast` | ipv4/unicast |
| `ipv6/unicast` | ipv6/unicast |
| `ipv4/multicast` | ipv4/multicast |
| `ipv6/multicast` | ipv6/multicast |
| `ipv4 mpls` | ipv4 mpls |
| `ipv6 mpls` | ipv6 mpls |
| `ipv4/mpls-vpn` | ipv4/mpls-vpn |
| `ipv6/mpls-vpn` | ipv6/mpls-vpn |
| `ipv4/flowspec` | ipv4/flowspec |
| `ipv6/flowspec` | ipv6/flowspec |
| `l2vpn/evpn` | l2vpn/evpn |
| `l2vpn/vpls` | l2vpn/vpls |

Special values: `all` (default), `none`

---

## Route Attributes

Attributes are flat keyword-value pairs (no `set` keyword). Both short and long forms accepted.
API text output uses short forms; config output uses long forms.

```
next <ip>                        # Next-hop IP (required) — long: next-hop
origin igp|egp|incomplete        # Origin attribute
path <asn>,<asn>,...             # AS path — long: as-path
pref <int>                       # Local preference — long: local-preference
med <int>                        # Multi-exit discriminator
s-com <comm>,<comm>,...          # Standard communities — long: community
x-com <ext>,<ext>,...            # Extended communities — long: extended-community
l-com <lc>,<lc>,...              # Large communities — long: large-community
originator-id <ip>               # Originator ID
cluster-list <ip>,<ip>,...       # Cluster list
label <label>                    # MPLS label (per-NLRI-section modifier)
rd <rd>                          # Route distinguisher (per-NLRI-section modifier)
info <id>                        # ADD-PATH path ID (per-NLRI-section modifier) — long: path-information
atomic-aggregate                 # Atomic aggregate flag
aggregator <asn> <ip>            # Aggregator
aigp <value>                     # AIGP
split /<len>                     # Ze: prefix expansion (see below)
```
<!-- source: internal/component/bgp/types/types.go -- RouteSpec, FlowSpecRoute -->
<!-- source: internal/component/bgp/types/nexthop.go -- RouteNextHop -->

### Keyword Aliases

| Long (config) | Short (API) | Also accepts |
|----------------|-------------|--------------|
| `next-hop` | `next` | `nhop` (legacy) |
| `local-preference` | `pref` | — |
| `as-path` | `path` | — |
| `community` | `s-com` | `short-community` |
| `large-community` | `l-com` | — |
| `extended-community` | `x-com` | `e-com` |
| `path-information` | `info` | — |
| `route-distinguisher` | `rd` | — |

Lists use commas (no spaces): `path 65001,65002`. Brackets accepted for transition: `as-path [65001 65002]`.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- keyword alias table -->

---

## Split Keyword (Ze Extension)

The `split` keyword expands a prefix into smaller prefixes. All attributes apply to each generated prefix.

### Syntax

```
split /<target-length>
```

### Example

```
# Announce 2 prefixes with one command
update text next 1.2.3.4 nlri ipv4/unicast add prefix 10.0.0.0/23 split /24
# → 10.0.0.0/24 next-hop 1.2.3.4
# → 10.0.1.0/24 next-hop 1.2.3.4

# With MPLS label - label applies to each prefix
update text next 1.2.3.4 nlri ipv4/nlri-mpls label 100 add prefix 10.0.0.0/22 split /24
# → 10.0.0.0/24 label 100
# → 10.0.1.0/24 label 100
# → 10.0.2.0/24 label 100
# → 10.0.3.0/24 label 100

# With L3VPN - RD and label apply to each prefix
update text next 1.2.3.4 nlri ipv4/mpls-vpn rd 100:1 label 200 add prefix 10.0.0.0/23 split /24
# → 10.0.0.0/24 rd 100:1 label 200
# → 10.0.1.0/24 rd 100:1 label 200
```

### Supported Families

| Family | Split Support | Notes |
|--------|---------------|-------|
| IPv4/IPv6 unicast | ✅ | Standard prefix expansion |
| IPv4/IPv6 nlri-mpls | ✅ | Label copied to each prefix |
| IPv4/IPv6 mpls-vpn | ✅ | RD + label copied to each prefix |
| FlowSpec | ❌ | N/A - uses match rules, not prefixes |
| VPLS/EVPN | ❌ | Different NLRI structure |

### Constraints

- Target length must be longer than source prefix (e.g., /23 → /24, not /24 → /23)
- Maximum expansion: implementation-dependent (avoid /8 → /32)

---

## FlowSpec Commands

FlowSpec rules use the unified `update text` syntax. The match components are
the FlowSpec NLRI (`nlri ipv4/flow add <components>`); the action is carried as
a FlowSpec extended community declared with the `extended-community` attribute
before the `nlri` section (`discard` is sugar for `traffic-rate 0`).

```
# Match TCP traffic to 10.0.0.0/8 port 80 and drop it
update text extended-community discard \
  nlri ipv4/flow add \
  destination 10.0.0.0/8 \
  protocol tcp \
  destination-port =80

# Withdraw the same FlowSpec rule (match components identify it)
update text nlri ipv4/flow del \
  destination 10.0.0.0/8 \
  protocol tcp \
  destination-port =80
```

### Match Components

| Keyword | Description |
|---------|-------------|
| destination | Destination prefix |
| source | Source prefix |
| destination-port | Destination port |
| source-port | Source port |
| port | Any port |
| protocol | IP protocol |
| next-header | IPv6 next header |
| tcp-flags | TCP flags |
| icmp-type | ICMP type |
| icmp-code | ICMP code |
| fragment | Fragment flags |
| dscp | DSCP value |
| packet-length | Packet length |
| flow-label | IPv6 flow label |

### Actions (then)

| Keyword | Description |
|---------|-------------|
| accept | Accept traffic |
| discard | Drop traffic |
| rate-limit <bps> | Rate limit |
| redirect <rt> | Redirect to VRF |
| redirect-next-hop | Redirect to next-hop |
| mark <dscp> | Set DSCP |
| community [...] | Add community |
<!-- source: internal/component/bgp/plugins/cmd/update/update_text_flowspec.go -- FlowSpec NLRI parsing -->
<!-- source: internal/component/bgp/route/route_community.go -- ParseExtendedCommunities (discard, traffic-rate, redirect actions) -->

---

## Filter Callbacks

The engine sends `filter-update` callbacks to external plugin filters during
UPDATE processing. This is a callback RPC (engine to plugin), not a user command.

| Field | Type | Description |
|-------|------|-------------|
| `filter` | string | Filter name (declared at stage 1, dispatches to the right handler) |
| `direction` | string | `import` or `export` |
| `peer` | string | Peer IP address |
| `peer-as` | uint32 | Peer ASN |
| `update` | string | Text-format attributes and NLRI (only declared attributes) |

Response: `{"action":"accept"}`, `{"action":"reject"}`, or
`{"action":"modify","update":"<delta>"}` with only changed fields.

<!-- source: internal/component/plugin/server/server.go -- CallFilterUpdate builds the filter-update request -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnFilterUpdate handles it plugin-side -->

---

## Response Format

### Success (with serial prefix)

```json
{"type":"response","response":{"serial":"1","status":"done"}}
```

### Error

```json
{"type":"response","response":{"serial":"1","status":"error","data":"description"}}
```
<!-- source: internal/component/plugin/types.go -- Response struct -->

### Rejected rows: `errors` and `data`

A handler can answer with a row generator rather than a built payload. It can
reject a row while the walk continues. A consumer that reads the whole answer as
one string sees the rejected rows under a SIBLING key. A partial result therefore
renders, instead of collapsing into one error string.

| Answer | Rendering |
|--------|-----------|
| rows, no rejected row | `{"peers":[...]}`, or a bare `[...]` when the handler names no envelope. Unchanged |
| rows and rejected rows | `{"peers":[...],"errors":[...]}` |
| rejected rows, no envelope key | `{"data":[...],"errors":[...]}` |
| the handler names its envelope `errors` | refused, on this path and on the record path |
<!-- source: internal/component/plugin/types.go -- Records.MarshalJSON -->
<!-- source: pkg/plugin/rpc/collapse.go -- CollapseRecords, AnswerErrorsKey, AnswerDefaultKey, ErrReservedEnvelopeKey -->

`errors` appears only when a row was rejected. An ordinary answer keeps the shape
it had, and no consumer meets a key it has not seen. `data` is where the rows go
when the handler names no envelope and a row was rejected. A bare array has
nowhere to carry a sibling.

`| count` counts the rows the command produced and never the rejected ones, so
the two collections stay separately countable. A commit that applied 97 leaves
and rejected 3 renders both, rather than the 97 being lost with the error.

Two kinds of handler answer with a generator. `system command list` is the
engine's, and a plugin command handler is the SDK's. Every other command returns
a built payload, and none of these keys appears for it.
<!-- source: internal/component/plugin/server/system.go -- commandRows -->
<!-- source: pkg/plugin/records.go -- Records -->

### A plugin command answers with records

`execute-command` is the callback the engine sends for a command a plugin
registered. Every plugin answers it with a head, its records and a terminator,
and the engine reads that sequence for every plugin.
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- Plugin.answerExecuteCommand -->
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn.SendExecuteCommandAnswer -->

The handler decides what it produces, and the wire decides how it travels.

| The handler returns | On the wire | What `routeToProcess` builds |
|---------------------|-------------|------------------------------|
| a built value | the `doc` item type and one record carrying that value | `plugin.RawJSON`, unchanged |
| a `plugin.Records` walk of 256 rows or fewer | the `doc` item type and one record carrying the collapsed document | `plugin.RawJSON` over that document |
| a `plugin.Records` walk of more than 256 rows, declaring no columns | the `map` item type and one record for each row | `plugin.Records` over the arriving rows |
| a `plugin.Records` walk of more than 256 rows, declaring its columns | the `tab` item type, the names on the head, and one positional record for each row | `plugin.Records` over the arriving rows, carrying the head's names |

The dispatcher branches on the head's item type and never on what the handler
returned. A bounded walk is therefore the document it has always been, and only a
walk that streams reaches an operator as records.

A `tab` answer reaches the operator as the same objects a `map` answer would
have carried. The engine forwards the head's column names beside the rows, and
the rendering zips each positional row against them. A command that declares a
schema and one that does not therefore answer one document for the same data.
<!-- source: internal/component/plugin/server/command.go -- streamedPluginResponse -->
<!-- source: internal/component/command/render_records.go -- RenderRecords, answerDocument -->
<!-- source: pkg/plugin/records.go -- Records.WriteAnswer -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswer, WriteDocumentAnswer -->
<!-- source: internal/component/plugin/server/command.go -- routeToProcess, pluginAnswerRows -->

The value a built payload carries is unchanged, byte for byte. Only the frame
around it changed, and it changed for every peer. Nothing declares an answer
shape, so there is one frame and every reader knows it before the first line.

The rows are pulled as the operator's rendering writes them, so the engine never
holds the whole collection for a walk that streams. A row wider than one wire
message is rejected and the walk continues. That row reaches the operator under
`errors`, beside the rows that were applied.
<!-- source: internal/component/plugin/server/system.go -- handleSystemCommandList, commandRows -->

### Show Neighbor

```json
{
  "neighbor": {
    "address": "192.168.1.2",
    "local-address": "192.168.1.1",
    "local-as": 65001,
    "peer-as": 65002,
    "router-id": "1.1.1.1",
    "state": "established"
  }
}
```

### Show Adj-RIB

```json
{
  "routes": [
    {
      "nlri": "10.0.0.0/8",
      "next-hop": "192.168.1.2",
      "origin": "igp",
      "as-path": [65002]
    }
  ]
}
```

---

## Command Dispatch

### Command Tree Structure

> **Note:** This tree shows the **internal noun-first dispatch structure**, not
> the user-facing grammar. User-facing commands are verb-first
> (`show`/`request`/`clear`/`update`/`monitor` roots, e.g. `show bgp peer <sel> detail`,
> `request cache forward`, `clear bgp rib in`); the noun-first RPCs below remain
> only for internal dispatch. Nodes such as `peer/<selector>/announce` and
> `peer/<selector>/withdraw` reflect removed verbs (see "Removed Commands").

```
daemon
├── shutdown
├── reload
├── restart
└── status

plugin
└── session
    ├── ready
    ├── ping
    └── bye

bgp
├── help
├── command
│   ├── list
│   ├── help
│   └── complete
├── event
│   └── list
├── log
│   ├── levels            # Show subsystem log levels
│   └── set               # Set subsystem log level at runtime
├── metrics
│   ├── values            # Show Prometheus metrics (text format)
│   ├── list              # List metric names
│   └── pool              # Per-attribute pool occupancy and dedup rates
└── plugin
    ├── encoding
    ├── format
    └── ack

peer
├── list
├── detail
├── capabilities
├── statistics
└── <selector>
    ├── detail
    ├── capabilities
    ├── statistics
    ├── teardown
    ├── announce
    ├── withdraw
    └── group

rib
├── routes [sent|received|sent-received] [filters...] [count|json]
├── best [filters...] [count|json]
├── status
├── clear [in|out]
├── inject <peer> <family> <prefix> [attrs...]
└── withdraw <peer> <family> <prefix>

group
├── start
└── end

monitor
├── bgp                   # Live peer dashboard (TUI)
└── event                 # Stream live events (keeps session open)
```

---

## Ze Implementation Notes

### Command Dispatcher

```go
type Handler func(ctx *Context, peers []string, remaining string) error

type DispatchTree map[string]interface{}  // Handler or nested DispatchTree

func Dispatch(tree DispatchTree, tokens *Tokenizer, reactor *Reactor) (Handler, []string) {
    // Walk tree consuming tokens
    // Return handler and matched peers
}
```
<!-- source: internal/component/plugin/server/command.go -- Dispatcher, Handler -->

### YANG-Typed Command Arguments

Operational commands declare their argument types as YANG leaves inside
`ze:command` containers. The same leaf metadata drives three consumers:

1. **Completer** (`command/completer.go`): enum values become tab-completion
   suggestions; keyword leaf names appear as completable tokens.
2. **Dispatcher** (`plugin/server/command.go`): validates args against ArgDefs
   between tokenize and handler call (two-phase: keyword extraction, then
   positional matching).
3. **Documentation**: leaf descriptions provide help text.

```go
type ArgDef struct {
    Name       string         // YANG leaf name (kebab-case)
    Kind       ArgKind        // ArgString, ArgEnum, ArgUint, ArgUnion
    EnumValues []string       // Valid enum values
    UintBits   int            // 8, 16, 32, or 64
    Ranges     []UintRange    // Valid ranges (disjoint segments supported)
    Pattern    *regexp.Regexp // Compiled XSD pattern for ArgString
    UnionDefs  []ArgDef       // Member types for ArgUnion
    Mandatory  bool           // True if YANG leaf has mandatory true
    Anchor     string         // Path keyword this value follows; "" for a trailing value
}
```

ArgDefs are extracted from YANG by `BuildCommandTree` (`config/yang/command.go`)
and stored on `command.Node.ArgDefs`. The dispatcher receives them via
`RegisterOptions.ArgDefs` populated by `PathToArgDefs`.

A container that names an object declares the value the operator types after
its keyword, once, and every command under it takes that value:
`request interface <name> up`, `<name> down`, `<name> mtu <bytes>` share one
`name` leaf on the `interface` container. `inheritArgDefs`
(`config/yang/command.go`) carries such a leaf down to each command after every
module is merged, with `Anchor` set to the container's name, and the renderer
places the value right after that keyword. The command under such a container
that acts on no single member of the set states `ze:inherit "none"`:
`show bgp peer list` reads every peer, and `request interface migrate` names two
interfaces of its own. Nothing binds a value by `Anchor`: a positional token
still goes to the definition whose type constrains it most (`positionalDef`).

Runtime-dynamic hints (e.g., address families from plugin registry) remain as
`ValueHints` callbacks. Static hints (log levels, FD limit "max") are
YANG-declared and served through ArgDefs.
<!-- source: internal/component/command/node.go -- ArgDef, Node.ArgDefs -->
<!-- source: internal/component/config/yang/command.go -- extractArgDefs -->

### Plugin Command Completion

Plugin-registered commands (from the plugin `CommandRegistry`, not YANG) are
absent from the YANG-derived command tree, so they are **injected into the
interactive completion tree after the daemon starts**.
`CommandRegistry.VisibleCommandEntries()` returns every non-`Hidden` command as a
`command.CommandEntry{Name, Description}`; `command.MergeCommandPaths` inserts
each path into the tree as completion-only nodes. The merge is non-destructive:
an existing YANG node keeps its `WireMethod` and description, so a plugin command
never shadows a builtin at the completion layer (mirroring dispatch precedence).

- **SSH** rebuilds the tree per session and merges eagerly
  (`session_factory.go` `mergePluginCommands`), so each session reflects the
  current registry — a plugin that has exited is simply absent next session.
- **Web** builds a throwaway overlay from the current registry for each
  `/cli/complete` request (`web_completer.go` `pluginAwareCommandCompleter`).
  The shared YANG tree stays immutable. The hub binds the web listener after
  plugin startup and the initial registry freeze. The per-request overlay
  reflects plugins that a later reload adds or removes.
- **Shell completion** (`ze completion words`) runs in a standalone CLI process
  with no daemon, so it stays YANG-only; the daemon's `system command complete`
  RPC completes plugin commands directly from the registry (`Registry().Complete`).

`Hidden` commands still dispatch when typed in full. They never appear in
completion, in help, in the MCP `tools/list` result, or in the API command list. <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
`buildCommandMeta` is the one producer of the last two surfaces, and it skips
every hidden plugin command.

<!-- source: internal/component/plugin/server/command_registry.go -- VisibleCommandEntries, Complete, Hidden -->
<!-- source: cmd/ze/hub/command_meta.go -- buildCommandMeta hidden plugin command skip -->
<!-- source: internal/component/command/node.go -- MergeCommandPaths, CommandEntry -->
<!-- source: cmd/ze/hub/session_factory.go -- mergePluginCommands (SSH per-session) -->
<!-- source: cmd/ze/hub/web_completer.go -- pluginAwareCommandCompleter (web live overlay) -->
<!-- source: cmd/ze/hub/main.go -- runYANGConfig -->
<!-- source: internal/component/plugin/server/startup.go -- signalStartupComplete, WaitForStartupComplete -->

### Quiesce Barrier (test synchronization)

`request quiesce` (`ze-system:quiesce`) **blocks until every registered
subsystem has drained its pending asynchronous work, then replies** — a barrier
tests use in place of a fixed `time.sleep`. It is the general form of
`ze-bgp:peer-flush`: the control plane is already synchronous (a command reply
lands after its handler runs), but downstream effects (routes flushed to peer
sockets, and later FIB/tc/listeners) complete after the reply, so a test does
`send(change); request quiesce; assert on-wire` with no sleep.

Subsystems register a `Quiescer` at runtime (they need a live reference such as
the reactor), and the handler discovers them through the registry, with no
per-subsystem switch. The BGP reactor auto-registers TWO quiescers when it
attaches to the server (`registerReactorQuiescer`): `bgp-forward-pool` (the
reactor's `FlushForwardPool`, draining post-establishment forwarded routes) and
`bgp-peer-sync` (`DrainPeerSync`, draining each peer's initial-sync opQueue,
which goes DIRECT to the session and bypasses the forward pool). Each drain is
bounded by a per-subsystem timeout, so a wedged subsystem yields an error naming
it instead of hanging the daemon.

Invocation note: `request quiesce` and `request peer <sel> flush` are api-yang
RPCs reached through **dispatch-command**, not as direct wire methods. A plugin
calls `api.dispatch("request quiesce")` (the test SDK's `ze_api.quiesce()` and
`ze_api.wait_for_ack()` both do this); a raw `_call_engine("ze-system:quiesce")`
returns "unknown method" because `dispatchPluginRPC` routes only
`ze-plugin-engine:*` engine ops plus codec RPCs.

Extension point: Layer 2/3 subsystems (a kernel-FIB quiescer, a tc/qdisc
quiescer) register into the same registry and `request quiesce` drains them with
no change to the barrier. `wait_for_ack` is now a thin, sleepless wrapper over
this barrier: the two BGP quiescers together cover the forward pool AND the
per-peer initial-sync drain, so a route sent during establishment is on the wire
(past its EOR) before the barrier returns.

<!-- source: internal/component/plugin/server/quiesce.go -- Quiescer, QuiescerRegistry, quiesceAll, handleQuiesce, registerReactorQuiescer -->
<!-- source: internal/component/bgp/reactor/reactor_api.go -- DrainPeerSync, peersSynced; peer.go PendingSync -->
<!-- source: internal/core/ipc/yang/ze-system-cmd.yang -- request/quiesce -> ze-system:quiesce -->

### Peer Selector Parsing

```go
type Selector struct {
    All       bool
    IP        netip.Addr
    Filters   map[string]string  // local-as, peer-as, local-ip, id, family
}

func ParseSelector(s string) (*Selector, error) {
    if s == "*" {
        return &Selector{All: true}, nil
    }
    if strings.HasPrefix(s, "[") {
        return parseFilteredSelector(s)
    }
    ip, err := netip.ParseAddr(s)
    return &Selector{IP: ip}, err
}
```

### Command Registry

```go
var Commands = []CommandInfo{
    {"daemon shutdown", false, nil},
    {"peer * update text", true, []string{"next", "origin", ...}},
    // ...
}
```
<!-- source: internal/component/plugin/server/rpc_register.go -- registeredRPCs -->

### Per-command declarations: what a command says about itself

Five registries let a command say something about itself to the CLI. Each one is
keyed by command path. Each resolves a command to the declaration registered on
the longest path that is a prefix of it. So `show bgp rib best` picks up a
declaration on `show bgp rib` unless it registers one of its own. A command that
must NOT inherit registers an EMPTY declaration. Absent and empty are different
answers, and that is the trap all five registries exist in.

| Registry | Declares | Read by |
|----------|----------|---------|
| `RegisterShape` | whether the answer holds rows (`tab`, `map`) or one document (`doc`) | `validateDeclaredShape`, before the command runs, and `ze help command --json` |
| `RegisterColumns` | the order the table and text renderers put this command's columns in | `tableStyle.orderKeys`, through the four `ProcessPipes*` wrappers |
| `RegisterAddressFields` | that a field of the answer holds an IP address | `validateDeclaredShape`, which admits `\| resolve` and `\| origin` only where a field is declared |
| `RegisterPipeFilters` | the pipe segments this command accepts as its own, which `foldFilters` rewrites into server-side arguments | `foldFilters`, the completer, the pipe validator |
| `RegisterAliases` | a name an operator types in the operator slot, standing for a chain (see "Pipe aliases" below) | `lookupAlias` |

One implementation resolves all five: `commandRegistry[T]` in `column_order.go`.

<!-- source: internal/component/command/answer_shape.go -- RegisterShape, ShapeForCommand, RegisterAddressFields, AddressFieldsForCommand -->
<!-- source: internal/component/command/column_order.go -- commandRegistry, lookup -->

#### Two packages, one path: empty is a floor and a disagreement is a defect

The first four registries are a `declarationRegistry[T]`, which adds one rule to
that lookup. `declare` reads what the path already holds:

| The value | The path holds | Result |
|-----------|----------------|--------|
| anything | nothing | the value is stored |
| EMPTY | anything | what the path holds stays |
| non-empty | an EMPTY declaration | the value replaces it |
| non-empty | an equal value | no change |
| non-empty | a DIFFERENT non-empty value | `panic("BUG:")`, naming the registry, the path and both values |

An empty declaration is a FLOOR and never a claim. It stops a shorter path being
inherited, and it says nothing about what the answer holds. `show bgp rib` is the
case this rule was written for. The BGP peer command plugin blanks every direct
child of `show bgp`, and the rib command plugin declares `tab` for
`show bgp rib`. Under the earlier last-writer-wins rule, package initialization
order decided which of the two the path answered.

Every in-tree caller declares from `init()`, so two different non-empty values
are a state only a Ze defect reaches. The panic reports it before the daemon
serves anything (`docs/contributing/ze-go-style.md`).

A plugin declares from a socket instead, and a bad declaration there is an
operating error rather than a Ze defect. So the first three registries carry a
second write, `declareFor`, which is the same four cases with the panic replaced
by an error. `RegisterPluginShapes` calls it, and nothing a plugin sends reaches
`declare`.

The ALIAS registry is the fourth, and it keeps `register`.
`RegisterPluginAliases` stores `mergedAliases(path, ...)`, which differs from
what the path holds each time it runs, so the rule would fire on the ordinary
case.

<!-- source: internal/component/command/column_order.go -- declarationRegistry, newDeclarationRegistry, declare, declareFor -->
<!-- source: internal/component/command/answer_shape.go -- RegisterPluginShapes -->
<!-- source: internal/component/command/alias.go -- aliasRegistry, mergedAliases -->

#### The declared shape refuses an operator before the command runs

`validateDeclaredShape` reads the declared shape and refuses an operator that
shape cannot support, by name, before dispatch. A command declaring `doc`
answers `count cannot apply here: this command answers one document, and count
acts on rows`. The answer's own shape refuses as well, at apply time, and that
half covers every command including the ones that declare nothing. The
declaration is what makes the published catalog true, because `ze help command
--json` lists a declared command's operators from its shape.

Both halves of that message are derived, because one operator needs more than
rows. `| fill` brings back the columns a command declared. So it acts on `tab`
alone, and it means nothing over a `map` answer whose rows carry their own keys.
`shapeDescription` therefore calls `map` "rows that describe themselves". It
calls `tab` "rows read against a declared column order", and `operatorNeeds`
reads the operator's own shape set. Calling both "rows" gave a refusal that
contradicted itself, in front of an operator whose answer HAS rows.

A declared address-field list is an ADMISSION gate AND a selector. It decides
whether `| resolve` and `| origin` run at all, and it decides what they
decorate. `bindAddressFields` copies the declaration onto each address operator
when the chain is parsed, so a later registry withdrawal cannot change an
in-flight chain, and `resolveJSON` and `originJSON` decorate a key only when
`addressFieldSelected` finds it in that list.

Standalone stdin is the one path that decorates every key.
`ProcessStandalonePipesChecked` sets `allAddressFields` on each address
operator, because stdin has no command path and therefore no declaration to
read. There `addressFieldSelected` returns true for every key whose value parses
as an address.
<!-- source: internal/component/command/pipe.go -- bindAddressFields, ProcessStandalonePipesChecked -->
<!-- source: internal/component/command/pipe_resolve.go -- resolveJSON, addressFieldSelected -->
<!-- source: internal/component/command/pipe_origin.go -- originJSON -->

Every `show bgp` command declares a shape, and two channels write them. Go
compiled into the daemon declares sixteen paths. Nine of those name an address
field. `show bgp rib` and `show bgp irr` are served by a plugin process, and an
in-core shim declares for them. Scope is therefore the registration site rather
than the process boundary.

A plugin process declares the other eleven in its Stage 1 message. Six sit under
`show bgp rpki`, two under `show bgp rs`, two under `show bgp adj-rib-in`, and
`show bgp healthcheck` is the eleventh. Four of the eleven name an address
field. See "A plugin declares its own answer shape" below.

A SELECTOR spelling declares nothing of its own. The registry resolves the string
the operator typed. `show bgp peer detail` is not a prefix of
`show bgp peer 192.0.2.1 detail`, so that spelling resolves `show bgp peer`, one
of the empty declarations. It therefore reaches no refusal before dispatch, and
the answer's own shape refuses after it.

<!-- source: internal/component/command/pipe.go -- validateDeclaredShape, shapeDescription, operatorNeeds -->
<!-- source: internal/component/command/pipe_resolve.go -- resolveJSON -->
<!-- source: internal/component/command/pipe_origin.go -- originJSON -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- cmdBgpChildren, registerShapes -->
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- commandDecls -->
<!-- source: internal/component/bgp/plugins/rs/server.go -- commandDecls -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- commandDecls -->
<!-- source: internal/component/bgp/plugins/healthcheck/healthcheck.go -- commandDecls -->

A column order never enters the payload. It is captured when the formatter is
built, from the command string the wrapper already holds, and it reaches
`| table` and `| text` only. `| json`, `| ndjson` and `| yaml` keep their
alphabetical keys, because a program reads those three and key order carries no
meaning for a program.

A command declares one order per record shape. `show bgp` renders an
outer record and a list of peer rows. Both carry an `uptime` key in a different
position. So the renderer applies the declaration that names the most of the
keys in the record it has in hand.

### `| display` and `| fill`: the operator's own answer

An operator overrides both halves of that with two generic pipe operators.
`| display <field>...` names the fields the answer leads with.
`| fill [alpha] [reverse]` says whether the fields it did not name come back at
all, and in what sequence. Each takes ONE type of argument, so no token is a
field name in one position and a keyword in another.

`| fill` on its own orders the remaining fields by the command's own
declaration. `alpha` orders them by field name instead. `reverse` flips
whichever way is in force. Neither way measures a column: each decides the
sequence from the key set and a declaration.

A third way was removed on 2026-08-19. `| fill overall` ordered columns by the
width they render at, and that width is known only after every cell of the whole
answer has been rendered. It made the first row unwritable until the last row
had been read, which is the one thing a streamed answer cannot do. `overall` is
now refused by name, and `| fill` itself is untouched.

The two halves of the request travel by different routes, and the split is what
makes them work under every format:

| Half | Where it is applied | Reaches |
|------|--------------------|---------|
| Selection: which fields | `applyDisplaySelect`, over the payload, at the operator's position in the chain | every format, `\| json`, `\| ndjson`, `\| yaml` and `\| raw` included |
| Sequence: in what order | `columnRequest` carried on `tableStyle` | `\| table` and `\| text` only |

Selection is a data question the operator asked out loud, so a program gets the
answer. Sequence is presentation, so it stops at the two renderers.

`selectFields` walks the same shapes `tableStyle.renderValue` walks, and applies
the same rule `orderKeys` applies. A record that carries at least one displayed
field is cut to the displayed ones. A record that carries none is left whole.
Without that agreement a nested sub-table and the JSON behind it would answer
with different fields. Without it a record naming nothing displayed would render
as a box with no rows.

**A kind the `foldFilters` switch does not name stays in the chain.** The switch
names the five kinds a command can own as a filter it resolves itself. Its
`default:` arm carries every other kind to the chain `ApplyPipes` runs over the
answer. Both sides run in the daemon. That arm is load-bearing. Without it a
kind named nowhere reached neither side, for every command that registers
filters of its own, and nothing reported the loss.
`TestColumnOpsSurviveFoldFiltersOnFilteredCommand`,
`TestAliasSurvivesFoldFiltersOnFilteredCommand` and
`test/ui/display-fill-filtered-command.ci` are what hold that.

<!-- source: internal/component/command/column_order.go -- RegisterColumns, ColumnsForCommand, commandRegistry -->
<!-- source: internal/component/command/pipe_filter.go -- RegisterPipeFilters, lookupPipeFilters -->
<!-- source: internal/component/command/pipe_columns.go -- parseDisplay, parseFill, columnsInChain, applyDisplaySelect, selectFields -->
<!-- source: internal/component/command/pipe_table.go -- tableStyle.orderKeys, fillKeys, bestColumnOrder -->

### Pipe aliases: a name for an operator chain

An alias is a name an operator types in the operator slot, standing for a chain
they would otherwise retype. `show bgp | peers` says what
`show bgp | display peers` says.

Two callers declare one, and both write the same registry.

| Registration | Table | Resolved |
|--------------|-------|----------|
| `RegisterAliases([]string{"show bgp"}, ...)`, from Go compiled into the daemon | `aliasRegistry`, the same `commandRegistry[T]` the two registries above use | by the longest command path that is a prefix of the command |
| `RegisterAliases(nil, ...)`, from the same Go | `globalAliases`, a table of its own | for every command, when the per-command lookup carries no alias of that name |
| `RegisterPluginAliases(owner, commands, declared)`, from a plugin's Stage 1 message | `aliasRegistry`, on the command paths that plugin declared | the same longest-prefix rule. A plugin reaches no global table |

The global table is separate rather than a registration on the empty command
path. `commandRegistry.register` skips an empty path, and
`commandMatchesPrefix` refuses an empty prefix against every non-empty command,
so such a registration would match nothing and report nothing.

`expandAliases` runs between `ParsePipe` and `foldFilters`, so classification
only ever sees operators the parser already knows. It is ONE pass, and four
properties are what make one pass enough. `checkAlias` is the one reading of all
four, so the two callers can never disagree about which declarations are sound:

- An alias MUST NOT name another alias. Its expansion parses to pipe operators
  alone.
- An alias MUST NOT carry the name of a pipe operator, which `ParsePipe` would
  read first.
- An alias MUST NOT carry the name of a pipe filter of an overlapping command
  path. A command's own filter resolves before anything generic, so the filter
  would win at use time and nothing would say why. `RegisterPipeFilters` refuses
  the same pair from its side, because package init order decides which of the
  two registrations runs second.
- An alias takes no argument. A word after the name is refused when the chain
  runs, rather than dropped.

The two callers differ in the ANSWER, not in the checks. `checkedAlias` turns a
refusal into `panic("BUG:")`. Only Go in this repository reaches
`RegisterAliases`, so a bad registration there is a Ze defect the compiler
cannot see. `RegisterPluginAliases` returns the refusal as an error, because the
strings it reads arrived over a socket.

An alias never enters the payload. `expandAliases` replaces the name with the
operators before the chain runs, so a command handler cannot tell an alias from
the chain it stands for. The chain is expanded in the process that PARSES it,
and for a plugin's alias that process MUST be the daemon. Read "Where an alias
resolves, and where it does not" below.

`show bgp` registers the two in-tree aliases that exist. Both are a selection
among sibling keys, because that answer carries its aggregates and its `peers`
array at the same level:

| Alias | Expands to |
|-------|-----------|
| `summary` | `display router-id local-as uptime peers-configured peers-established` |
| `peers` | `display peers` |

The BGP RPKI plugin declares a third, `summary` on `show bgp rpki`, over the
Stage 1 channel the next section describes.

<!-- source: internal/component/command/alias.go -- RegisterAliases, RegisterPluginAliases, checkAlias, checkedAlias, lookupAlias, AliasesForCommand -->
<!-- source: internal/component/command/pipe.go -- parsePipeChain, expandAliases, parsePipeOps -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- registerAliases -->

### A plugin declares a pipe alias in its Stage 1 message

`DeclareRegistrationInput.Pipes` is a list of `PipeDecl`, beside the lists that
carry families, commands, filters, doctor checks and enrichers. The SDK
re-exports the type, so an external author imports one package. Each entry
carries four strings, and the engine parses the expansion once, at registration.

| Field | Meaning |
|-------|---------|
| `command` | The command path the alias sits on. It MUST be one of the commands this plugin declares in the same message |
| `name` | The word an operator types after the pipe character. Lowercase kebab-case, one word |
| `description` | The line completion and `command help` show beside the name |
| `expansion` | The operator chain the name stands for, written the way an operator would type it |

`validatePipeDecls` reads the shape and the ownership, in the position where
Stage 1 already validates doctor checks and enrichers, before it converts
anything. `registerPluginPipes` then writes the accepted set under
`startupRegistrationMu`, between the registry row and the runtime families. Each
later failure unwinds what the steps above it wrote.

A plugin names only a path it declared itself, and it reaches no global table.
A path another PLUGIN declared is refused a step earlier: `PluginRegistry.Register`
runs before the alias write and rejects a command name another plugin holds, so
the second plugin fails on the COMMAND and never reaches its alias.

What the check confirms is that the plugin DECLARED the path, not that the
daemon routes that path to it. A plugin that declares a name the daemon serves
itself, `show bgp` for one, passes here. The dispatcher's own registry rejects
that command entry later as a builtin conflict, and the plugin keeps running, so
its alias sits on a command path the daemon answers. It can only ADD a name
there, never take one: the exact-path check refuses a name the path already
carries, `mergedAliases` keeps what the path held, and the name leaves with the
plugin. Declaring a name a builtin already serves is a plugin defect, and the
daemon logs it as `command registration rejected ... conflicts with builtin`.

<!-- source: pkg/plugin/rpc/types.go -- PipeDecl, DeclareRegistrationInput -->
<!-- source: internal/component/plugin/registration.go -- PluginRegistry.Register -->
<!-- source: internal/component/plugin/server/command_registry.go -- CommandRegistry.Register, AddBuiltin -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- PipeDecl -->
<!-- source: internal/component/plugin/server/startup.go -- validatePipeDecls, registerPluginPipes, commandPathKey -->

#### Collision has two populations, because there are two resolution rules

Reading collision as "any overlapping path" refuses the case the channel exists
for. `show bgp` carries an alias named `summary`, and every `show bgp *` path
overlaps `show bgp`, so that reading refuses `show bgp rpki | summary` before it
is written.

| Pair | Collides when | Why |
|------|---------------|-----|
| Alias against alias | the two sit on the SAME normalized command path | `lookupAlias` reads the set on the longest registered prefix and never falls back to a shorter one. A longer path SHADOWS a shorter one, and that is how `show bgp rpki` answers `summary` while `show bgp` answers one of its own |
| Alias against pipe filter | their command paths OVERLAP | `foldFilters` resolves a command's own filter for the whole subtree the filter covers, so an overlapping filter makes the alias unreachable |
| Alias against a built-in operator name | always | `ParsePipe` reads the built-in name first, so the alias is never reached |

`aliasOnPath` is the exact-path reading and `filterShadowing` is the overlapping
one. A declaration is also refused when one name appears twice on one path in
one message. A declaration naming a command path the plugin did not declare is
refused too.

A refusal fails the whole Stage 1 registration, and the plugin does not start.
Nothing is registered when any one declaration is refused, so a plugin never has
to undo a partial registration. The message names the plugin, the command path
and the alias name. The daemon log is where an operator reads it, because the
engine stops a refused plugin before it can report anything itself.

<!-- source: internal/component/command/alias.go -- aliasOnPath, filterShadowing, RegisterPluginAliases -->

#### A declaration ADDS to a path. It never replaces what the path holds

`commandRegistry.register` stores one value for each path, so writing a declared
set straight into the registry drops every alias that path already answered to.
`show bgp rpki` already carries the empty declaration the in-tree BGP command
plugin puts on every child of `show bgp`. A plugin therefore declares onto an
occupied path in the ordinary case. `mergedAliases` adds the declared names to
what the path holds. Every declared name is checked against that same set first,
so nothing merged this way replaces anything.

Removal is by ENTRY for the same reason. `UnregisterPluginAliases` takes back the
names one owner registered, and leaves the in-tree names and other owners' names
in place. A path the owner created from nothing goes once its last entry is gone.
Without removal a plugin that stops cannot start again, because the exact-path
check then refuses it its own name.

Two call sites remove it, and they are the two that remove the registry row and
the runtime families. `rollbackStartupProcess` is both the failed-startup path
and the config-reload stop path. The other is the family-conflict unwind inside
`onRegistration`.

<!-- source: internal/component/command/alias.go -- mergedAliases, UnregisterPluginAliases, pluginAliasPath -->
<!-- source: internal/component/plugin/server/startup.go -- rollbackStartupProcess, engineStartupSink -->

#### The engine derives the barrier that stops an alias below its command

An alias on `show bgp rpki` is inherited by `show bgp rpki roa`, because `roa`
registers nothing of its own and the lookup resolves the longest registered
prefix. That offers the name on a leaf whose answer cannot carry it.

`aliasBarriers` derives the answer from the plugin's own command list. It reads
every command the plugin declared that sits strictly below a path carrying one
of its aliases. Each such command that declares no alias itself gets an empty
declaration. The in-tree form of the same barrier is written by hand in
`peer.go`, over `cmdBgpChildren`. A plugin author writes nothing and does not
have to know the resolution rule.

<!-- source: internal/component/command/alias.go -- aliasBarriers -->
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- cmdBgpChildren -->

#### The pipe layer selects and re-sequences. It cannot compute

This is the obligation every command that wants an alias owes, and it is a
property of the PAYLOAD rather than of the pipe layer.

`display` keeps the named keys and drops the rest. `fill` re-sequences what
`display` did not name. `count` replaces the answer with a number. `first` and
`last` cut the item list. `match` keeps the rendered lines that match a pattern.
The format operators render one payload a different way.

None of them renames a key, adds two numbers, counts the rows whose field holds
a given value, or asks the handler for anything. So a command whose second view
is a pipe alias MUST EMIT the aggregate fields beside the detail rows, as
siblings at one level. `show bgp` has always done this, and `show bgp rpki` now
does it too: `overviewCommand` writes `appendSummaryFields` and
`appendCacheServers` into one record, and `| summary` selects the first half.

Four of the seven RPKI aggregate fields are computed. `vrp-count` sums the two
family counts, `sessions-established` counts the sessions in one state,
`sessions-total` renames what `show bgp rpki status` calls `sessions`, and
`validation-enabled` is a constant. The command computes them, and the alias
only selects them.

The expansion is a second copy of the field names, and `display` names keys and
reports no miss. A field added to the payload and not to the expansion is
therefore dropped from the alias in silence. A conversion owes three things:

- one authored list of the field names.
- an expansion built from that list rather than repeating it.
- a test holding the list against the bytes the writer produces.

RPKI does this with `summaryFieldNames` and `buildSummaryAliasExpansion`.

Two questions send a candidate back to being a subcommand. Would the operator
have to supply a value? An alias takes no argument. Does the answer need data the
parent's payload does not carry? An alias reshapes what was returned.

`show bgp rpki roa 192.0.2.0/24` fails both and stays a subcommand.
`show bgp rpki cache` fails the second one: it reports `preference`,
`session-id`, `serial` and three intervals that the bare answer does not carry.

<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- overviewCommand, appendSummaryFields, appendCacheServers, summaryFieldNames, buildSummaryAliasExpansion -->

#### Where an alias resolves, and where it does not

The chain is resolved in the process that parses it, and a plugin's alias lives
in the daemon's registry alone.

| Surface | Parses the chain | A plugin's alias works |
|---------|------------------|------------------------|
| `ze cli -c "<command>"`, and any SSH exec channel | the daemon, in `execMiddleware` | Yes |
| The TUI a plain ssh client with a pty reaches | the daemon, which hosts the Bubble Tea model | Yes |
| `ze cli` with no command argument | the CLIENT process, in `executeOperationalCommand` | No |
| `cliClient.StreamMonitor`, for a streaming monitor command | the CLIENT process | No |

On the two client-side rows the operator reads
`pipe error: unknown pipe operator: <name>`, and Tab offers the name nowhere.
The aliases compiled into the client resolve there, which is what hid the gap.
Tab after the pipe character on `show bgp` offers `summary` and `peers` in the
same client. The repair is a wire surface that carries the daemon's alias table
to the client at session start, and it is NOT built.

<!-- source: internal/component/ssh/ssh.go -- execMiddleware -->
<!-- source: internal/component/cli/model_mode.go -- executeOperationalCommand -->

#### Discovery: the running daemon is the only source

A surface that reads the compiled tree in its own process starts no plugin, so
it never sees a plugin's Stage 1 message.

| Surface | Reads | Reports a plugin's alias |
|---------|-------|--------------------------|
| `command help "<name>"` | the running daemon's registries | Yes, as a `pipe-aliases` list beside `pipe-filters` |
| Tab completion in the daemon-hosted TUI | the running daemon's registries | Yes |
| `./le command list` | the compiled tree in its own process | No, and it cannot |
| `ze help command --json`, and the wiki catalog built from it | the compiled tree in its own process | No, and it cannot |

`command help` lists an in-tree alias and a declared one the same way, and it
listed neither before 2026-08. It reports the expansion beside the description.
An alias takes no argument and names no other alias, so the chain it stands for
is the whole of what the name does.

The same table governs a plugin's declared answer shape. `ze help command
--json` and the wiki catalog built from it read the compiled tree in their own
process. So they list a plugin's commands without the operators the plugin
declared. The running daemon refuses and publishes correctly. The published
catalog cannot yet say so.

<!-- source: internal/plugins/meta/cmd/help.go -- commandHelp, pipeAliasHelp, handleBgpCommandHelp -->
<!-- source: internal/le/command/list/commandlist.go -- Answer -->
<!-- source: cmd/ze/help_command.go -- collectCommands, extractPipes -->

#### A plugin declares its own answer shape

Each `CommandDecl` in the Stage 1 message carries three optional fields:
`shape`, `columns` and `address-fields`. An absent field is an undeclared field,
so a plugin written before this channel existed keeps its old behavior.

| Field | Holds |
|-------|-------|
| `shape` | `doc`, `map` or `tab`, the same three words the answer head uses on the wire |
| `columns` | the answer's keys, in the order a person reads them. It needs a shape with rows |
| `address-fields` | the keys whose value holds an address. It needs a shape |

`validateShapeDecls` reads the three fields where `validatePipeDecls` reads the
aliases. It refuses four declarations by name:

- a spelling that is not one of the three words.
- a column or address-field list with no shape.
- a declaration on a blank command path.
- a list or a name past its bound.

The bounds are 64 columns and 16 address fields for one command, with each name
1 to 64 bytes. A refused declaration fails the plugin's startup and writes
nothing.

`registerPluginShapes` then writes under `startupRegistrationMu`, between the
pipe aliases and the runtime families, and joins the same unwind.
`UnregisterPluginShapes` takes the whole declaration back when the plugin stops.

**A plugin can never panic the daemon with a declaration.** The three registries
keep the panic on `declare`, which only Go compiled into the daemon reaches. The
plugin route is `declareFor`, which is the same four cases with the panic
replaced by an error. Two in-tree packages that disagree are still a Ze defect
and still panic.

Every declaration writes all THREE registries. A command that declares a shape
and no column writes an EMPTY column declaration. That empty declaration is the
barrier that stops the command inheriting its parent's order. Removal restores
the empty declaration a shim left behind, rather than deleting the path.

The declaration lives in the daemon's registry. So `| display <partial>` offers
a plugin command's field names in the daemon-hosted session, and offers none in
`ze cli` with no command argument. That is the same client-side gap the alias
table above records, for the same reason.

<!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->
<!-- source: internal/component/plugin/server/startup.go -- validateShapeDecls, validateDeclaredFieldName, registerPluginShapes -->
<!-- source: internal/component/command/answer_shape.go -- RegisterPluginShapes, UnregisterPluginShapes -->
<!-- source: internal/component/command/column_order.go -- declarationRegistry, declare, declareFor, withdraw -->
<!-- source: internal/component/command/completer.go -- completeDisplayFields, completePipeForCommand -->
<!-- source: internal/component/bgp/plugins/cmd/rib/rib.go -- registerPipeFilters, registerRibAnswerShapes -->
<!-- source: internal/component/bgp/plugins/filter_irr/cmd_irr.go -- registerIRRShapes -->

### The chain over a row generator

A handler that answers with a row generator runs the same chain, one record at a
time. `applyPipesRecords` is the record half of `ApplyPipes`. `| match`,
`| count`, `| first`, `| last`, `| display`, `| resolve` and `| origin` each act
per record, so `| count` holds nothing and `| last 8` holds eight records.
<!-- source: internal/component/command/pipe_records.go -- applyPipesRecords, applyRecordOp, recordsCounted, recordsLast -->

A format operator changes no record. `RenderRecords` renders what the chain
produced. It writes per record for one chain alone: `| ndjson`, over an answer
that declares no column schema, whose chain folded no display metadata.
Every other format needs a document. A column width needs every row, and metadata
rides in the envelope.
<!-- source: internal/component/command/render_records.go -- RenderRecords, streamsPerRecord, writeDocument -->

`| table` and `| text` therefore collect. That cost is paid once, in the
renderer, and the record stage forwards the records untouched.

A chain that answers a document of its own is not filed under the command's
envelope. `| count` is the one operator that does: it answers `{"count":N}`
whatever it counted, and the whole-payload path replaces the payload for the
same reason. `system command list | count` therefore answers `{"count":N}`, the
same document every other command's `| count` answers, and not
`{"commands":[{"count":N}]}`.
<!-- source: internal/component/command/render_records.go -- chainAnswersItsOwnDocument, answerDocument -->
<!-- source: internal/component/command/pipe.go -- applyCount -->
<!-- rfc: none -- this is Ze's own command surface -->
<!-- since: 2026-08-20 -->

Authorization is decided once, at dispatch, and the rows are produced after that
decision. That is what a built payload has always done, and a generator changes
only how long the gap is. There is no per-row authorization, so a handler MUST
NOT yield a row the caller was not already authorized to receive when the
command was accepted.
<!-- source: internal/component/plugin/server/dispatch.go -- dispatchCommandArgsResponse -->
<!-- source: internal/component/plugin/server/system.go -- commandRows -->

A chain the validator refuses answers one rejected row and pulls nothing, so an
unreadable chain never reads as an empty answer.
<!-- source: internal/component/command/pipe_records.go -- faultRecords -->
<!-- source: internal/component/command/pipe.go -- ValidatePipes -->

---

## Managed Config RPCs

RPCs for hub-client managed configuration. These operate over MuxConn after auth,
separate from the plugin 5-stage protocol.

| Verb | Direction | Payload | Response |
|------|-----------|---------|----------|
| `config-fetch` | Client to hub | `{"version":"<hash-or-empty>"}` | `{"version":"<hash>","config":"<base64>"}` or `{"status":"current"}` |
| `config-changed` | Hub to client | `{"version":"<hash>"}` | `{}` |
| `config-ack` | Client to hub | `{"version":"<hash>","ok":true}` or `{"version":"<hash>","ok":false,"error":"..."}` | `{}` |
| `ping` | Either direction | `{}` | `{}` |

Version hash is truncated SHA-256 (16 hex characters) of config bytes.

<!-- source: pkg/fleet/envelope.go -- RPC payload types -->
<!-- source: internal/component/plugin/server/managed.go -- hub-side handlers -->
<!-- source: internal/component/managed/client.go -- client-side handlers -->

---

## MCP Methods

The MCP Streamable HTTP transport (revision `2026-07-28`) is stateless and
strictly client-to-server. Every message is its own HTTP POST, and the server
never sends an independent JSON-RPC request on any stream. These methods are
distinct from ze's own dispatcher commands (above). They are part of the MCP
protocol contract, and each request carries the version and capabilities it
speaks in its own `params._meta`.

| Method | Direction | Purpose |
|--------|-----------|---------|
| `server/discover` | Client -> server | Advertise `supportedVersions`, `capabilities` (including `extensions["io.modelcontextprotocol/ui"]` for MCP Apps and `extensions["io.modelcontextprotocol/tasks"]` for background tasks), and `instructions`. Mandatory for a server to implement, and optional for a client to call |
| `tools/list` | Client -> server | The tool inventory, derived from the command registry at call time, in a deterministic order. A descriptor carries `_meta.ui` only when the request declared the `io.modelcontextprotocol/ui` extension compatibly | <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
| `tools/call` | Client -> server | Run a tool. Answers `resultType: "task"` when the command's `ze:task-support` annotation is `required` and the request declared the tasks extension. Answers `resultType: "input_required"` when the tool needs a value the call did not supply | <!-- doc-links: ignore (JSON-RPC method name, not a path) -->

<!-- source: internal/component/mcp/streamable_tools.go -- runMethod dispatch switch -->
<!-- source: internal/component/mcp/discover.go -- serverDiscover -->
<!-- source: internal/component/mcp/mrtr.go -- newInputRequiredResult, permitsInputRequired -->

There are no server-initiated methods. `notifications/tasks/status` existed
under the earlier revision and was removed with the session and the GET stream it
required.

`elicitation/create` survives, but no longer as a method. `elicitation/create`
is now a **value** inside the `inputRequests` map of an `InputRequiredResult`. A
server RETURNS that result from `tools/call` (or `resources/read`), and the <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
server does not send it. The client then retries the original request with
`inputResponses`. See [MCP Elicitation](../../guide/mcp/elicitation.md) for the
full round trip.

See [MCP Architecture Overview](../mcp/overview.md#capability-negotiation)
for how capabilities are declared per request.

## MCP Task Methods

These are the `io.modelcontextprotocol/tasks` extension, not core protocol. A
`tools/call` on a command annotated `ze:task-support required` creates a <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
background worker and returns a `CreateTaskResult` immediately. The client then
polls for status, and the server pushes nothing.

| Method | Direction | Purpose |
|--------|-----------|---------|
| `tasks/get` | Client -> server | Current state of a task by `taskId`. A terminal task carries its outcome here: `result` when completed, `error` when failed |
| `tasks/update` | Client -> server | Answer a task's outstanding input requests. Ze raises none, so it verifies ownership and acknowledges with an empty result, ignoring unknown `inputResponses` keys |
| `tasks/cancel` | Client -> server | Request cancellation of a working task |

`tasks/list` and `tasks/result` were REMOVED this revision and are now unknown
methods, answered HTTP 404 with `-32601`. A poll of `tasks/get` replaced the
blocking `tasks/result`. That change is why a terminal state carries its
payload. `tasks/list` was dropped outright. No method enumerates tasks now, so a
client tracks the ids it was given.

All three surviving methods require the request's
`_meta["io.modelcontextprotocol/clientCapabilities"]` to declare
`io.modelcontextprotocol/tasks` under `extensions`. The bare `tasks` member that
the earlier revision used is no longer accepted. A request without the
declaration is refused with `-32021` (`MissingRequiredClientCapability`) and
HTTP 400, carrying `data.requiredCapabilities` in the extension shape.

<!-- source: internal/component/mcp/tasks.go -- task registry -->
<!-- source: internal/component/mcp/streamable_tools.go -- tasksGet, tasksUpdate, tasksCancel, failMissingTasksCapability -->
<!-- source: internal/component/mcp/meta.go -- parseClientCapabilities -->

Task creation is server-directed. There is no `task` member on `tools/call` <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
params. The server decides per tool from the YANG `ze:task-support` annotation:
`required` always, `forbidden` never, and `optional` synchronous. A client that
did not declare the extension gets the ordinary synchronous result, not an
error.

<!-- source: internal/component/mcp/streamable_tools.go -- callTool, createTask -->
<!-- source: internal/component/mcp/tools.go -- groupTaskSupport -->

## MCP Resource Methods

<!-- source: internal/component/mcp/resources.go -- resources/list, resources/read -->

| Method | Direction | Description |
|--------|-----------|-------------|
| `resources/list` | Client -> server | List all available UI resources. The response comes from an embedded FS walk |
| `resources/read` | Client -> server | Read a single resource by `ui://` URI. Returns content as text or base64 blob |

Both results carry cache hints (see MCP Result and Error Envelope below).

Neither method is gated on a client capability. `resources` is a member of
`ServerCapabilities`, not of `ClientCapabilities` (whose members are
`experimental`, `roots`, `sampling`, `elicitation` and `extensions`). No
conformant client can therefore declare `resources`. A server that advertises
the capability in `server/discover` serves it.

<!-- source: internal/component/mcp/resources.go -- resourcesList, resourcesRead -->
<!-- source: internal/component/mcp/meta.go -- clientCapabilities -->

## MCP Result and Error Envelope

Every successful MCP result carries a `resultType` and
`_meta["io.modelcontextprotocol/serverInfo"]`, stamped from one shared helper so
no method can omit them. `resultType` is `complete` for a finished result.
`resultType` is `input_required` for the MRTR interim result a handler produces
when it needs a value the request did not supply. The shared helper preserves
`input_required` and does not overwrite it. And a guard on the single path out
of dispatch refuses to emit `input_required` on any method other than
`prompts/get`, `resources/read` and `tools/call`. <!-- doc-links: ignore (JSON-RPC method name, not a path) -->

<!-- source: internal/component/mcp/streamable_tools.go -- ok, resultMeta, runMethod -->
<!-- source: internal/component/mcp/mrtr.go -- guardInputRequired, permitsInputRequired -->

Four methods additionally carry the `CacheableResult` fields, `ttlMs` and
`cacheScope`. Both are non-optional on those results.

| Method | `ttlMs` | `cacheScope` |
|--------|---------|--------------|
| `server/discover` | `60000` | `private` |
| `tools/list` | `60000` | `private` | <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
| `resources/list` | `3600000` | `private` |
| `resources/read` | `3600000` | `private` |

`tools/call` and every `tasks/*` method carry neither field, in either result <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
shape. Three reasons make that correct. `tools/call` is absent from the <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
specification's cacheable-operation list. Interim `input_required` results are
explicitly not cacheable. And a result produced by an MRTR retry must not be
cached at all.

Ze therefore applies the hints from a per-method table on the way out of
dispatch. The shared `ok()` responder does not apply them, because `tools/call` <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
also uses it.

`cacheScope` is `private` on every cacheable result, which forbids a shared
gateway or caching proxy from serving one authorization context's response to
another. It is defence in depth, not the access control: per-request
authentication remains the gate.

<!-- source: internal/component/mcp/caching.go -- cacheTTLByMethod, stampCacheHints -->

| Code | HTTP | Meaning |
|------|------|---------|
| `-32020` | 400 | `HeaderMismatch`: a required standard header is missing or disagrees with the body |
| `-32021` | 400 | `MissingRequiredClientCapability`: `data.requiredCapabilities` names what the client must declare |
| `-32022` | 400 | `UnsupportedProtocolVersion`: `data.supported` lists the server's versions, `data.requested` echoes the client's |
| `-32602` | 400 for a malformed `params._meta`, 200 otherwise | Invalid params |
| `-32601` | 404 | Unknown method, including `initialize` |
| — | 405 | GET or DELETE to the MCP endpoint |

<!-- source: internal/component/mcp/streamable.go -- handlePOST, httpStatusForDispatch -->
<!-- source: internal/component/mcp/headers.go -- validateStandardHeaders -->
<!-- source: internal/component/mcp/streamable_tools.go -- failUnsupportedVersion -->

Three HTTP request headers are mandatory on every POST, and each one must agree
with the body:

- `MCP-Protocol-Version` mirrors the `_meta` protocol version.
- `Mcp-Method` mirrors `method`.
- `Mcp-Name` mirrors `params.name` for `tools/call` and `prompts/get`, and it <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
  mirrors `params.uri` for `resources/read`.

Ze decodes the `=?base64?...?=` sentinel in `Mcp-Name` first, then compares the
decoded value with the body.

<!-- source: internal/component/mcp/headers.go — mcpNameSource -->

Tool descriptors in `tools/list` carry `_meta.ui.resourceUri` when the command <!-- doc-links: ignore (JSON-RPC method name, not a path) -->
group has a `ze:ui-resource` YANG extension. The `_meta.ui` block is emitted
unconditionally, and the `ui://` asset it points at is readable by every caller.

---

**Last Updated:** 2026-07-29
