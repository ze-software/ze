# Spec: fleet-0 -- Fleet Management (Umbrella)

| Field | Value |
|-------|-------|
| Status | ready |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Depends | - |
| Phase | - |
| Updated | 2026-11-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/fleet-config.md` -- existing fleet config architecture
4. `internal/component/plugin/server/managed.go` -- hub-side managed config service
5. `internal/component/managed/` -- client-side managed lifecycle
6. `pkg/fleet/` -- shared RPC types and version hashing
7. Child specs: the Phase rows of the two Child Specs tables below, phases 1 to 10, each named by its path (`plan/release/features/` holds phases 1, 4, 8 and 10, `plan/post-release/features/` phases 2, 3, 5 and 9; phases 6 and 7 are closed)

## Task

This umbrella assigns outcomes to the child specs. It introduces no separate server, protocol implementation, database or command registration. Where a dated direction update disagrees with a later ruling, the later ruling and the owning child define the shipping behavior. The June proposals below preserve the history of those choices.


Extend Ze's fleet management from a config-distribution mechanism into a usable fleet
operations platform. The foundation is solid: TLS transport, per-client auth, config
fetch/push, transactional commits with rollback, health checks, rich hardware inventory,
and Prometheus metrics all exist at the single-device level. The work spans device reporting, a hub registry and operator commands:
collecting, storing, querying, and acting on fleet-wide state.

The registry already answers which declared devices are online and which versions they reported. An operator managing 10–1,000 devices still needs estate health, group changes, a history of who acted and staged rollouts. This spec set addresses those gaps incrementally, each child
spec building on the previous.

### Existing Foundation

| Capability | Package | Status |
|------------|---------|--------|
| TLS 1.3 transport, MuxConn RPC | `pkg/plugin/rpc/`, `internal/component/plugin/ipc/tls.go` | Implemented |
| Per-client token auth | `internal/component/plugin/ipc/tls.go` (AuthenticateWithLookup) | Implemented |
| Config fetch/push (1:1) | `pkg/fleet/`, `internal/component/managed/`, `internal/component/plugin/server/managed.go` | Implemented |
| Heartbeat + reconnect with backoff | `internal/component/managed/heartbeat.go`, `reconnect.go` | Implemented |
| Local ZeFS config cache | `internal/component/managed/handler.go` | Implemented |
| Connected device state and durable history | `ManagedServer.devices`, `ManagedRegistrySnapshot` and the fleet-1 record store | Implemented |
| Per-device health registry | `internal/core/health/` | Implemented |
| Hardware inventory (CPU, NIC, DMI, storage, SMART) | `internal/component/host/inventory.go` | Implemented |
| ~20 Prometheus collectors | `internal/component/telemetry/collector/` | Implemented |
| Transactional config commits with rollback | `internal/component/config/transaction/` | Implemented |
| Config archive (named destinations, triggers) | `internal/component/config/archive/` | Implemented |
| Appliance OTA push (--all --parallel N) | `cmd/ze/appliance/cmd_push.go` | Implemented |
| Appliance config push via SSH | `cmd/ze/appliance/cmd_config_push.go` | Implemented |
| Web fleet peer selector | `internal/component/web/page_system.go` (CollectFleetPeers) | Implemented (UI-only, config-declared peer URLs) |

### Design Principles

| Principle | Detail |
|-----------|--------|
| Extend hub, not new server | The existing plugin hub has TLS, auth, MuxConn, connection tracking. Fleet management adds hub-side state, not a separate fleet server. Consistent with learned decision 444 |
| New RPC verbs on existing transport | Fleet RPCs (inventory-report, health-report, etc.) are additional verbs on the same `#id verb [json]\n` framing. No new protocol |
| Hub-side persistent state in ZeFS | Device registry, audit log, and fleet state stored in the hub's ZeFS blob. Same storage as client configs. No external database |
| Config templates are config, not code | Fleet-2 declares template variables and group assignment in its own YANG surface. Operators manage them through the existing CLI editor and web config editor |
| Device reports, hub aggregates | Devices push inventory and health to the hub. The hub stores and exposes aggregated views. Devices remain self-contained; the hub is additive |
| Offline appliance toolchain unchanged | `ze appliance push/config-push` continues to work for SSH/HTTP-based operations. The runtime fleet protocol is a parallel path for always-connected devices |

### Scope

**In Scope:**

| Area | Description |
|------|-------------|
| Device registry | Persistent hub-side registry of all known devices with metadata, config version, health, last-seen |
| Fleet commands | `show fleet devices`, `show fleet device name <name>` on every command surface in the first release; the web pages listing fleet state are the `fleet-ui` set's, after it |
| Config templates | Named templates with variable substitution, group-based assignment |
| Device grouping | Labels/tags on devices, group-based targeting for config and operations |
| Audit trail | Centralized log of config pushes, acks, rejections, device connects/disconnects |
| Inventory + health aggregation | Devices report inventory and health upstream; hub stores and exposes aggregated views |
| Staged rollout | Percentage-based config push with automatic pause on ACK failure |

**Out of Scope:**

| Area | Reason |
|------|--------|
| PKI/certificate lifecycle | Separate concern; the managed listener already serves a configured PKI leaf or one issued by the daemon authority. A managed client verifies the server against its trust roots before sending its own token. Device enrollment and certificate-bound client identity remain outside this set |
| Multi-hub replication | Explicitly a non-goal (learned 444). HA via client cached config |
| Zero-touch provisioning (ZTP) | Requires DHCP option integration and unsigned bootstrap. Separate spec |
| Firmware distribution | Covered by existing `spec-cpe-5-firmware-update.md` (version check) and `ze appliance push` (OTA delivery) |
| Maintenance windows | Requires alerting integration not yet in Ze. Future spec |
| Fleet-wide rollback | Achievable as staged rollout with 100% targeting + previous config version. No separate mechanism needed |
| External metrics aggregation | Operators already use Prometheus. Fleet provides the scrape target list, not a replacement metrics store |
| Web per-device HTTP proxy | The accepted November relay carries dispatcher commands and answers over the authenticated managed session (Fleet-9). No child proxies HTTP sessions |
| Dynamic enrollment + device identity | Runtime device approval and mTLS / cert-bound identity replacing the pre-declared shared secret (extends the PKI/cert-lifecycle row). Deferred; the shared secret is sufficient for the freeze/resolution step |
| Bidirectional config sync | Continuous two-way merge. Replaced for now by single-writer plus an operator-gated `config-push` at divergence (`fleet-7`); full bidirectional sync stays out of scope |

### Child Specs

The 2026-08-29 move treated the fleet set as optional capability. Fleet
management now ships in the first release (`ai/META-DECISIONS.md`, "Goal and
scope"): the device registry (phase 1), device health reports (phase 4),
metric history (phase 8) and configuration drift detection with repair by the
hub (phase 10) live in `plan/release/features/`, beside the fleet interface's
alarm list (`spec-fleet-ui-5-alarm-list`, closed 2026-11-27; documented in `docs/architecture/fleet-alarms.md`)
and alarm causes (`plan/release/features/spec-fleet-ui-8-alarm-causes.md`).
Phases 2, 3, 5 and 9 stay in `plan/post-release/features/`. Phases 6 and 7,
the configuration freeze and the reconnect resolution, are closed. This
umbrella stays in `plan/post-release/features/`.

Thomas ruled on 2026-11-23 (voice-transcribed answers "1 b" and "2 b",
`ai/META-DECISIONS.md`, "Goal and scope") that the first-release fleet
features are reached through commands on every command surface (CLI, SSH, MCP,
REST, gRPC, the web command line), and that their web pages and views come
after the first release. Every web criterion of phases 1, 4, 8 and 10 moved to
the post-release `fleet-ui` spec that owns its page: the `/fleet` pages, the
seam's split into parts and the estate part to
`plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md`,
dashboards and charts to `spec-fleet-ui-3-dashboards-and-charts.md`, the
device System tab to `spec-fleet-ui-4-device-tabs-and-console.md`, the device
Config tab to `spec-fleet-ui-6-config-rollout-audit-admin.md`, the task model
and panel, notifications, live page updates and the alarm pages to
`spec-fleet-ui-9-tasks-notifications-and-alarm-pages.md` (split from
`fleet-ui-5`, whose alarms stay in the first release). Each child names what
moved in its Task section. Where a row below says "web", that part is the
`fleet-ui` spec's, after the first release.

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `spec-fleet-1-device-registry` (closed 2026-11-25; design in `docs/architecture/fleet-config.md`, "Device registry") | Hub-side persistent device registry. `show fleet devices` and `show fleet device name <name>` on every command surface. Device metadata (name, config version, last-seen, labels; health is `fleet-4`'s). Labels on the hub's client entry. The fleet web pages are `fleet-ui-2`'s, after the first release | - |
| 2 | `plan/post-release/features/spec-fleet-2-config-templates.md` | Named config templates with variable substitution. Group-based assignment. The device fetches its admitted rendered version. YANG schema for templates and groups | fleet-1 |
| 3 | `plan/post-release/features/spec-fleet-3-audit-trail.md` | Centralized audit log for fleet operations: config push/ack/reject, device connect/disconnect, template changes. Commands on every command surface; the audit web views are `fleet-ui-6`'s. Structured log entries in ZeFS | fleet-1 |
| 4 | `spec-fleet-4-inventory-health` (closed 2026-11-26; design in `docs/architecture/fleet-config.md`, "Inventory and health reporting") | New `inventory-report` and `health-report` RPC verbs. Devices push on connect and periodically. Hub stores per-device inventory and health. `show fleet device inventory name <name>`, `show fleet device health name <name>` and `show fleet health` on every command surface. The inventory and health web views are `fleet-ui-2`'s, `fleet-ui-3`'s and `fleet-ui-4`'s, after the first release | fleet-1 |
| 5 | `plan/post-release/features/spec-fleet-5-staged-rollout.md` | Percentage-based config push targeting device groups. Rollout state machine (pending, rolling, paused, complete, failed). Automatic pause on ACK failure threshold. CLI rollout commands | fleet-2, fleet-4 |
| 6 | `spec-fleet-6-config-freeze` (closed 2026-11-19; design in `docs/architecture/fleet-config.md`, "The baseline and the hold" and "The freeze") | Persisted baseline and reconnect hold (no silent stomp); `request fleet disable` and `enable` with immediate sever, `show fleet status`; best-effort freeze of operator edits on a managed device; declared-client guard on every hub writer. See the Direction Update of 2026-11-19 | - |
| 7 | `spec-fleet-7-config-reconnect-resolution` | Resolve a `diverged` device: the config-held verb on the existing TLS channel, a diff at the hub (Local=hub, Remote=router), adopt or revert, and its own per-client record of the pushed config. Closed 2026-11-19; design in `docs/architecture/fleet-config.md`, "The baseline and the hold" ("The hub's copy of a hold") | fleet-6 |

Phases 2, 3, and 4 are independent of each other (all depend only on fleet-1) and can proceed in parallel. Phase 5 depends on templates (for group targeting) and health reporting (for rollout health gates). Phases 6 and 7 are the direction update below: 6 (freeze, persisted baseline, safe reconnect hold) depends on nothing new and is safe to land first; 7 (diverged-config resolution) depends on 6 and surfaces through `show fleet divergence` and the `diverged` field of `fleet-1`'s `show fleet devices` (`fleet-1` AC-13); its web banner and diff are `fleet-ui-6`'s, after the first release.

### Direction Update (2026-06-26): Single-Writer Config, Emergency Disable, Reconnect Resolution

This is the June proposal. The November 19 update records the actual baseline key, hold verbs and local-edit policy. Current commands and the certificate choice are documented in the managed configuration architecture.

A review of the fleet direction settled the open transport/auth/sync questions and added two
child specs. Key reframing: the proposed permanent SSH tunnel from router to hub is an
implementation choice, not a new capability. Ze already has a persistent, authenticated,
NAT-traversing channel (the managed TLS connection), so the new requirements are additive on top
of it; SSH is not adopted, as it would mean building reverse-tunnel support Ze does not have.
Decisions:

| Topic | Decision |
|-------|----------|
| Transport | Keep TLS MuxConn; no SSH tunnel |
| Auth | Keep pre-declared per-client shared secret; enrollment/PKI stays out of scope |
| Local edits on managed devices | Frozen (single-writer), not bidirectional sync |
| Emergency change | `ze fleet disable` leaves the fleet and unfreezes; severs the connection immediately |
| Reconnect with divergence | Hold and let the operator decide via a commit-style diff; never auto-stomp |

Model: `meta/instance/managed` drives both fleet membership and a local-edit freeze. Config for
a managed device is single-writer -- the hub writes while the device is connected (device
frozen), the device writes while disconnected (hub frozen for that device). The device persists a
**baseline** (`meta/instance/managed/base-version`, the hash of the last config it received from
the hub). On reconnect it compares its active config to that baseline: equal -> normal resync and
apply any pending hub update; different -> it was locally edited, so the device holds (keeps
running its config, never auto-applies) and is marked `diverged` for the operator to resolve. The
baseline is required, not optional: `cfg.Version` is recomputed from the live config at startup
(`ze_core_start.go`), so without a persisted baseline the device cannot tell a local edit
from a pending hub update. Detection and the hold live in `fleet-6`; resolution (the commit-style
diff and a new `config-push` up-verb on the existing TLS transport) is `fleet-7`. `config-push`
is a bounded, operator-gated up-channel, NOT the continuous bidirectional sync that remains out
of scope.

Interaction with existing children:
- `fleet-5` (staged rollout) targets only connected devices, since the hub cannot change a
  disconnected device's config (you cannot stage a change for an offline device).
- `fleet-1` (device registry) gains a `diverged` device state, surfaced on the dashboard and
  consumed by `fleet-7`.

Caveat: single-writer is enforced at the editor level. Raw `ze data write` to the blob bypasses
both the device-side and hub-side guards, so a determined operator can still create a two-sided
divergence. The persisted baseline keeps even that case safe (the device holds rather than
stomps); guarding the blob write path is possible future work.

### Direction Update (2026-11-18): Fleet Web Interface, Command Relay, Metric History

The owner asked for a web interface to manage the estate and settled its scope.
The interface is a spec set of its own, `fleet-ui`
(`plan/post-release/features/spec-fleet-ui-0-umbrella.md`). Two rows of the
Out of Scope table above are superseded, and two children are added to this set.

| Topic | Decision |
|-------|----------|
| Web views | Every fleet web view is specified in the `fleet-ui` set. Where a child of this set names a web page or web view (`fleet-1` the `/fleet` page, `fleet-3` and `fleet-4` their web views, `fleet-5` and `fleet-7` their dashboard surfaces), that page is owned by `fleet-ui`; the child keeps the data, the protocol and the CLI commands the page reads |
| Web per-device command routing | Now in scope, as `fleet-9`. The mechanism is a relay of commands over the managed channel, not HTTP proxying |
| Relay identity | A relayed command runs under an authorization profile the device's own configuration names. No grant, no relay |
| External metrics aggregation | Now in scope for one bounded purpose, as `fleet-8`: the hub keeps about a week of a fixed sample set, persisted and bounded, for its own dashboards. The hub is still not a metrics store, and Prometheus stays the detailed source |
| Hierarchy | Unchanged: labels stay flat key=value pairs. The interface builds its tree by grouping on labels, and `group` stays the unit that assigns a template |
| Scale | The hub holds each managed connection under a cap of 128 for its life (`managedMaxConns`, `internal/component/plugin/server/managed_serve.go`). An estate of the size this umbrella targets cannot connect under it. Lifting it is owed by `fleet-1` |

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 8 | `spec-fleet-8-metric-history` (closed 2026-11-27; design in `docs/architecture/fleet-config.md`, "Device metric sampling and hub-side history") | Devices report a fixed 15-figure sample set over `sample-report`; the hub keeps a bounded, persisted dual-resolution history and answers `show fleet sample latest`, `show fleet sample series`, and `show fleet sample storage` on every command surface | fleet-4 |
| 9 | `plan/post-release/features/spec-fleet-9-device-command-relay.md` | The hub relays an operator's command to a device over the managed channel, under a profile the device grants, with streaming, deadline, cancellation and audit on both sides | fleet-1, fleet-3 |
| 10 | `plan/release/features/spec-fleet-10-drift-hash-chain.md` | Added 2026-11-23. Hub and client fold every applied hub change into a SHA-1 chain and compare its head on every reply; a mismatch on a client that holds no local edit resyncs by diff, with a full replay as fallback. A local edit still holds for `fleet-7`'s adopt or revert. When both fail, a Configuration resync failed alarm opens through `fleet-ui-5`'s alarm kind registry | fleet-1, fleet-7, fleet-ui-5, fleet-ui-8 |

The command answers the interface reads are proposed by the sample answers of
the fleet interface's sample source, `internal/component/web/fleetui/sample`
(`docs/architecture/fleet-interface.md`). They are
input to the design of `fleet-1`, `fleet-4`, `fleet-8` and `fleet-9`: a child
that settles a different answer shape changes the samples in the same work.

### Direction Update (2026-11-19): fleet-6 Redesigned, Single-Writer Relaxed

`fleet-6` and `fleet-7` were judged against `main` on 2026-11-18 and both were
REWORK; each spec carries its disposition. `fleet-6` was redesigned and
approved by the owner. Where this section and the Direction Update of
2026-06-26 differ, this one holds.

| Topic | Decision |
|-------|----------|
| Hub writes while a device is away | Allowed. The hub may always write the config of a client it declares. The "hub frozen for that device" half of the single-writer model is dropped: it forbade provisioning a client, whose first config is written before it ever connects, and the delivery of a change to a client that is away. A hub and a departed device can therefore both change a config, and `fleet-7`'s diff resolves it |
| What keeps a local edit | The baseline and the reconnect hold, on the device. The freeze is best effort: about thirty routes write a device's config, so it tells the operator early and the hold is the guarantee |
| Baseline key | `meta/managed/base-version`. `meta/instance/managed/base-version` cannot exist: `meta/instance/managed` is a value, and the store refuses a key below a value |
| A device with no baseline | Clean: it follows its hub as before and records a baseline from then on |
| Commands | `request fleet disable`, `request fleet enable`, `show fleet status`, through the daemon on every surface with a local route. Not `ze fleet ...` |
| Rejoin | Live after a live leave. A daemon started unmanaged connects at its next start |
| A managed device whose config names no hub | Stays frozen; `request fleet disable` is the exit and needs no hub |
| How the hub learns of a hold | In `fleet-6`, through the existing refused acknowledgement, with no change to `pkg/fleet`. `fleet-7` gives the hold its own cause |
| `diverged` record on the hub | `fleet-7` keeps its own per-client record of the config a device pushed up; its presence is the flag. It does not wait for `fleet-1`, which later reads that record. This replaces "`fleet-1` gains a `diverged` device state ... consumed by `fleet-7`" above |
| `fleet-5` | Unchanged in effect: a staged rollout still targets connected devices, because that is where it can observe the result, not because the hub may not write for an absent one |

### Relationship to Existing Specs

| Spec | Relationship |
|------|-------------|
| `spec-cpe-5-firmware-update.md` | Complementary. Firmware version check reports to the fleet dashboard. Fleet-5 staged rollout could orchestrate firmware updates, but the check mechanism itself is independent |
| `spec-vrf-0-umbrella.md` | Independent. VRF is per-device topology; fleet is multi-device management. Fleet devices may run VRFs internally |

### Key Architectural Decisions

| Decision | Rationale |
|----------|-----------|
| Device registry in ZeFS blob, not SQL | Ze has no SQL dependency. the daemon already opens and owns the live ZeFS tree. Fleet-1 verifies its published device bound |
| Labels are flat key=value pairs | Simple, proven (Kubernetes, Prometheus). No hierarchical grouping needed at this scale |
| Config templates render on the hub | The client receives ordinary configuration bytes. The template and staged-rollout children specify when a version is admitted for a device, including reconnect. A fetch cannot release an unstarted batch |
| Audit log is append-only blob entries | Fleet-3 retains the newest configured number in the hub store. No operator command edits these entries. External log shipping is a separate concern |
| Staged rollout is hub-side orchestration | Client protocol unchanged. Hub controls which devices get notified and when. Rollout state persisted in ZeFS for crash recovery |
| Health reporting via new RPC verb, not Prometheus scrape | Hub needs structured health data (component-level status), not raw metrics. Prometheus remains for detailed metrics. Health is for fleet-level operational decisions |

### Post-wave corrections (2026-07-10)

New obligation from the 2026-07 implementation wave (verified against current code):

| Item | Detail | Citation |
|------|--------|----------|
| Write robustness landed on the plugin RPC transport | `pkg/plugin/rpc/conn.go` now applies a default 30s write deadline when the context has none (`defaultWriteDeadline`, conn.go; applied in `writeAppended`, conn.go, :309), and arms a fail-fast write watchdog on transports without `SetWriteDeadline` (fields conn.go, armed via `NewConn` at conn.go, path chosen at conn.go). A stalled write past the window logs, fires the hook (`SetWriteWatchdogHook`, conn.go), and closes the connection (`fireWatchdog`, conn.go) | conn.go, :91-93, :107, :139, :191-200, :286-334 |
| Metric wired hub-side | `ze_plugin_write_watchdog_total` (CounterVec, transport label) registered and hooked in `internal/component/plugin/server/server.go`; documented in `docs/plugin-development/metrics.md` | server.go |
| Managed connections take the deadline path | Both managed endpoints wrap TLS `net.Conn`s in `rpc.NewConn` (client: `internal/component/managed/client.go`; hub: `internal/component/plugin/server/managed_serve.go`), which support `SetWriteDeadline`, so fleet RPC writes get the 30s deadline; the watchdog timer itself never arms for them | client.go, managed_serve.go |

Implication for this umbrella: the "New RPC verbs on existing transport" design principle now
inherits this behavior. Children fleet-4 and fleet-7 must integrate it: a peer that stalls a
write for 30s now surfaces as a write error / closed connection instead of an indefinite
block, so new verb flows must treat deadline-triggered write failure as a normal
disconnect/reconnect path, and payloads must stay well within the 16 MB `MaxMessageSize`
frame bound (`pkg/plugin/rpc/framing.go`, enforced at write time in conn.go).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/hub-architecture.md` - the daemon built in `cmd/ze/hub`
- [ ] `docs/architecture/web-workbench-pages.md` - the workbench shell and its reusable domain-page components
- [ ] `docs/architecture/fleet-config.md` -- existing fleet config architecture
  -> Decision: extend hub, not new server. Config-as-identity. Two-phase config change
  -> Constraint: preserve existing protocol and architecture
- [ ] `docs/architecture/core-design.md` -- component lifecycle, event bus, registration pattern
  -> Constraint: fleet component follows registration pattern; publish only the events each child owns; fleet-1 needs no EventBus subscription
- [ ] `internal/component/plugin/server/managed.go` -- hub-side ManagedConfigService
  -> Decision: use the existing ManagedServer registry alongside its ManagedConfigService

**Key insights:**
- The named managed listener authenticates the device and the existing daemon owns its sessions. The registry persists device observations in the live tree. ZeFS blobs serve seed, backup and export.
- Five verbs currently carry config fetch, notification, ack, hold and ping. Children extend the existing multiplexed framing.
- The audit child distinguishes observations available on the hub from local mode changes that can first reach it after reconnect.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go`, `managed_serve.go`, `managed_registry.go` — the config reader, authenticated session owner and fleet-1 per-device history.
  → Constraint: resolve a requested device through the existing declared client set and its admitted session.
- [ ] `pkg/fleet/envelope.go` — config-fetch, config-changed, config-ack, config-held and ping.
  → Constraint: use the existing managed framing and preserve the locally held device's baseline.
- [ ] `internal/component/managed/client.go` — the device authenticates, fetches, heartbeats and consumes hub notices.
  → Constraint: a slow report or relay worker must leave the configuration path responsive.
- [ ] `internal/core/health/` -- health registry: component checks (healthy/degraded/down), aggregated report, HTTP endpoint
  -> Decision: health-report RPC sends the same structured data as the HTTP endpoint
- [ ] `internal/component/host/inventory.go` -- hardware inventory: CPU, NIC, DMI, memory, thermal, storage, SMART. Cached detection with TTL
  -> Decision: inventory-report RPC sends a subset (no raw SMART data; summary only)
- [ ] `docs/architecture/fleet-config.md` -- full architecture doc for existing fleet config implementation
  -> Constraint: all fleet-0 children must preserve the existing protocol and architecture described here

**Behavior to preserve:**
- Current config, ack, hold and heartbeat semantics remain available alongside the children’s declared extensions
- Per-client auth and config isolation unchanged
- Client cached config for partition resilience unchanged
- Appliance toolchain (ze appliance push/config-push) unchanged
- Web fleet peer selector (config-declared peer URLs) preserved alongside the `fleet-ui` pages

**Behavior to change:**
- Hub-side reporting and audit reuse the durable ManagedServer device registry
- Device labels as a `label` list on the hub's existing `client` entry in `ze-plugin-conf.yang`, with no `fleet` root (fleet-1); templates and groups in YANG (fleet-2, whose draft places them under a `fleet {}` root)
- New RPC verbs for inventory and health reporting (fleet-4)
- Config fetch extended to support template rendering (fleet-2)
- New commands under `show fleet` and `request fleet rollout` (fleet-1, fleet-4, fleet-5), each registered once and reached on every command surface
- No web page in the first release: the fleet web pages are the `fleet-ui` set's, after it (`fleet-ui-2` the estate tree and device tables, `fleet-ui-3` the dashboards, `fleet-ui-4` the device tabs, `fleet-ui-9` the task, notification and alarm pages)

## Existing Mechanisms (MANDATORY)

Added 2026-11-18, when the direction update above touched this spec. The rows
restate the Existing Foundation table of the Task section against the code as
it stands today; where that table names an older symbol or path, this one is
current.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Managed channel, hub side | `internal/component/plugin/server/managed_serve.go`, `ManagedServer` | Accepts managed devices over TLS, authenticates each by its per-client secret, serves `config-fetch`, pushes `config-changed`, and owns admitted sessions plus the recorded device history. Fleet-1 owns replacing the historic 128-connection admission limit | Extends: the registry (`fleet-1`), reports (`fleet-4`, `fleet-8`) and the relay (`fleet-9`) are added on this server, not on a second one |
| Managed channel, device side | `internal/component/managed/client.go`, `RunManagedClient` | Connects, fetches and caches its configuration, answers hub requests in one loop, heartbeats and reconnects | Fleet-6 and Fleet-7 already protect the baseline and upload a locally held configuration. Fleet-4/8 reports and Fleet-9 relay workers preserve that processing. Fleet-10 extends the client and hub with drift comparison and bounded repair for clean devices |
| Verbs and envelopes | `pkg/fleet/envelope.go` | The five current verbs are `config-fetch`, `config-changed`, `config-ack`, `config-held` and `ping` | Extends: new verbs on the same framing; public API, changed with `docs/architecture/fleet-config.md` |
| RPC transport | `pkg/plugin/rpc`, `MuxConn` | Multiplexed requests with streamed answers, a 16 MiB frame bound and a write deadline | Extends: used as it is by every new verb |
| Hub-held device configuration | Store key `file/active/client-<name>.conf`, read by `ManagedServer` | One configuration per device, served on fetch | Extends: templates (`fleet-2`) select a direct configuration or render for the device in the shared desired-configuration reader; the audit trail (`fleet-3`) records changes to it |
| Local baseline and hub-held divergence | `internal/component/managed/baseline.go`, `BaselineAccess`, `Diverged`; `internal/component/plugin/server/managed_held.go`, `ReadHeldRecord`; `internal/component/managed/cmd_divergence.go`, `adoptHeldLocked` and `revertHeldLocked` | A device protects changed active bytes and reports a hold. The running hub records the held version and the operator’s adopt/revert decision. | Fleet-3 journals those distinct outcomes; Fleet-10 yields to a hold before attempting drift repair. |
| Versioned device configuration | `internal/component/config/storage/history_revision.go`, `ListVersions` and `ReadVersion`; `pkg/fleet/version.go`, `VersionHash` | Dated entries point at content-addressed configuration bytes. | Fleet-10 uses those bytes to build its transport diff, verifies reconstructed bytes, and can replay a complete version. |
| Health registry | `internal/core/health` | Per-component state on each device, shown by `show health` | Extends: `fleet-4` reports it to the hub |
| Hardware inventory | `internal/component/host` | CPU, memory, interfaces, storage and platform facts on each device | Extends: `fleet-4` reports a summary to the hub |
| Bounded time series | `internal/component/l2tp/cqm.go`, the sample ring | Fixed buckets with configured retention, for L2TP quality | Extends: `fleet-8` uses the same shape for the hub's history |
| Command dispatcher | `internal/component/plugin/dispatch.go`, `CommandDispatcher` | Runs a command for a caller with authorization and audit on every surface | Extends: `fleet-9` supplies one bound to a device; every `show fleet` command registers into the same registry |
| Config transactions and archive | `internal/component/config/transaction`, `internal/component/config/archive` | Verify, apply and rollback of a configuration; archived versions | Extends: a pushed configuration is applied through it, and rollouts (`fleet-5`) rely on its refusal |
| State store | `ManagedServerConfig.Store` in `internal/component/plugin/server/managed_serve.go` and keys in `pkg/zefs/keys.go` | The managed server borrows the storage handle already opened by the daemon | Extends: the registry, audit log, rollout state and history are stored through it |
| Appliance push over SSH | `internal/appliance`, `cmd_push.go` and `cmd_config_push.go` | Software and configuration push from a build machine | Does not apply: an offline toolchain, kept as it is beside the runtime channel |
| Web selector of other devices | `internal/component/web/page_system.go`, `CollectFleetPeers` | Links to other devices' own addresses | Does not apply: it holds no state; the fleet web interface is `fleet-ui` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|---|---|---|---|---|---|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "`pkg/plugin/sdk`, `pkg/plugin/rpc`, `pkg/ze`, `pkg/zefs` and `pkg/fleet` are imported by code outside this repository." | agrees | The managed channel and its envelopes are a public contract. The owning child documents each addition and registers its command with the common dispatcher. |
| `docs/architecture/fleet-config.md` | Key Rules | Owned child contract | "The client name in `client edge-01 { }` IS the client's identity" | agrees | Per-client authentication binds the device name to this session; optional metadata and the new verb leave ordinary configuration and holds intact. |
| `docs/architecture/fleet-config.md` | Design decisions | Shared contract of the child specifications | "Thomas chose “Allow first provisioning and later edits from the device’s web Config tab”. An authorized operator can commit the first or a later version of a declared device’s configuration while it is offline, through the post-release fleet web surface." | agrees | UI-6 owns this post-release web editor. Both a declared device’s first desired configuration and later edits can be committed offline. Delivery waits for a managed connection and honors the hold. |
| `docs/architecture/pki/tls-listeners.md` | Decision: a named certificate fails closed | Owned child contract | "A configured name that does not resolve returns an error and no material." | agrees | An unresolved configured certificate prevents the handshake. Only an authenticated live session carries this operation. |
| `docs/architecture/fleet-config.md` | Non-Goals | Fleet-10 | "Full config on change; configs are small (< 10KB typically)" | departs | The drift-repair child explicitly owns the change to diff delivery with a bounded full fallback. |

## Data Flow (MANDATORY)

### Changed-Wire Census

The umbrella has no wire writers or readers. Its children own the actual protocol changes and identify the writers and readers in each child census. The existing public control is `pkg/fleet/envelope.go`; notably `config-held` is already in production source while Fleet-10 delivery and Fleet-9 relay remain child designs.

### Entry Point
- Device connects to hub via TLS (existing path)
- Hub-initiated config-changed notifications (existing path)
- New: device-initiated inventory-report and health-report RPCs (fleet-4)
- New: operator commands for fleet management, on every command surface

### Transformation Path
1. Device connects, authenticates (existing)
2. Hub registers device in persistent registry with metadata (fleet-1, admission and disconnect through ManagedServer)
3. Device requests config via config-fetch (existing)
4. The shared desired-config reader resolves the admitted version, preferring a direct configuration to the group template. Both fetch and push use that reader (Fleet-2/5).
5. Device reports inventory and health (fleet-4, new RPC handling)
6. Hub stores device state and answers it through commands on every command surface (fleet-1, fleet-4); the `fleet-ui` pages read the same commands after the first release
7. Operator initiates staged rollout (fleet-5), hub orchestrates notifications

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Device to Hub | TLS + MuxConn RPC (existing transport) | [ ] |
| Hub to ZeFS | Blob read/write for device registry, audit log, templates | [ ] |
| Hub to command surfaces | Builtin commands through the daemon dispatcher, the same answer on the CLI, SSH, MCP, REST, gRPC and the web command line | [ ] |
| Template to Config | Hub-side effective configuration read by fetch, push and version queries | [ ] |

### Integration Points
- `ManagedConfigService.HandleConfigFetch` and `BuildConfigChanged` share the desired-config reader; the serving session records the outcome
- `ManagedServer.handleConn` in `managed_serve.go` authenticates each declared device and owns registry admission and disconnect
- Live page updates -- none in the first release: `fleet-1` emits no EventBus fleet event, and the pages' live updates are the `EventSource` of `fleet-ui-9`, after it
- YANG schema -- labels on the existing `bus { hub { server { client {} } } }` entry (fleet-1); templates and groups as `fleet-2` designs them
- Command registry -- `show fleet`, `request fleet rollout`, registered once and reached on every command surface
- Web pages -- none in the first release: the estate tree and device tables are `fleet-ui-2`'s, the dashboard `fleet-ui-3`'s, device detail `fleet-ui-4`'s, rollout status `fleet-ui-6`'s, the task, notification and alarm pages `fleet-ui-9`'s, all after the first release

### Architectural Verification
- [ ] No bypassed layers (fleet uses existing TLS/auth/MuxConn path)
- [ ] No unintended coupling (fleet is additive; standalone devices unaffected)
- [ ] No duplicated functionality (extends ManagedConfigService, not parallel)
- [ ] Zero-copy preserved where applicable

## Acceptance Criteria

These are the top-level outcomes. Each child spec has its own detailed ACs.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|---|---|---|---|
| AC-1 | Hub with 10+ managed clients connected | `show fleet devices` lists all with name, config version, health status, last-seen | Authorized devices stay visible while their health is unreported; the status names that state. |
| AC-2 | Device disconnects and reconnects | Registry shows offline period, last-seen updates on reconnect | The retained history remains because a disconnect ends a session, not device identity. |
| AC-3 | Config template assigned to a group | Each group member without a direct override receives its admitted rendered version on an eligible fetch; its local hold still protects an emergency edit | Directly configured and locally held devices retain their specified precedence. |
| AC-4 | Operator pushes config change to a group | Audit trail records: who, when, which template, which devices, ack/reject per device | Offers, ack and rejection stay separate observations. |
| AC-5 | Device reports inventory to hub | `show fleet device inventory name edge-01` shows CPU, NIC, memory, storage summary | An offline device keeps its last report and receipt time. |
| AC-6 | Device reports degraded health | `show fleet device health name <name>` shows the component degraded and `show fleet health` counts the device degraded, on every command surface (`fleet-4` AC-4, AC-7, AC-13). The dashboard's Degraded count moved to `fleet-ui-3` AC-8, after the first release | An unreported device retains its own count. |
| AC-7 | Staged rollout at 20% with failure | Rollout pauses automatically; `show fleet rollout` shows pause reason and affected devices | Unstarted devices stay on the prior version even after force complete. |
| AC-8 | Standalone device (not managed) | Zero fleet overhead; no fleet YANG required; existing behavior unchanged | No managed connection or worker is created on a standalone device. |
| AC-9 | A device with stored samples and an active cause goes offline | Metric history shows the real gap; the last report remains queryable, the caused alarm links to Device offline, and recovery follows the kind's rule (Fleet-4, Fleet-8, UI-5 and UI-8). | Other devices and independently actionable alarms remain visible. |
| AC-10 | Hub and a device's applied configuration disagree | Fleet-10 repairs a non-holding device by verified diff or full transfer. Local divergence remains subject to adopt/revert. | A local hold continues to protect its active configuration. |
| AC-11 | After release an authorized hub caller sends a command to a device | Fleet-9 enforces that caller's target scope and the device's grant, returns the device’s answer, ends abandoned work and records its outcome. | An authorized read remains available under a read-only profile; a hub-side denial reaches only the hub audit. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Device connects to hub with per-client auth | -> | DeviceRegistry.Register persists device record | `test/managed/fleet-device-register.ci` |
| `show fleet devices` CLI command | -> | Fleet query returns all registered devices | `test/managed/fleet-show-devices.ci` |
| Config template assigned to device group | -> | The shared desired-config reader supplies the admitted rendered version to HandleConfigFetch | `test/managed/fleet-template-fetch.ci` |
| Device sends inventory-report RPC | -> | Hub stores inventory, answered by `show fleet device inventory name <name>` | `test/managed/fleet-inventory-report.ci` |
| Staged rollout initiated for group | -> | Rollout state machine progresses, pauses on failure | `test/managed/fleet-staged-rollout.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDeviceRegistryRegister` | `internal/component/plugin/server/registry_test.go` | Device registration persists to blob | |
| `TestDeviceRegistryLabels` | `internal/component/plugin/server/registry_test.go` | Label assignment and selector matching | |
| `TestConfigTemplateRender` | `internal/component/plugin/server/template_test.go` | Variable substitution in config templates | |
| `TestAuditLogAppend` | `internal/component/plugin/server/audit_test.go` | Audit entries appended to blob | |
| `TestInventoryReportHandler` | `internal/component/plugin/server/fleet_report_test.go` | Inventory RPC parsed and stored | |
| `TestHealthReportHandler` | `internal/component/plugin/server/fleet_report_test.go` | Health RPC parsed and stored | |
| `TestRolloutStateMachine` | `internal/component/plugin/server/rollout_test.go` | State transitions: pending, rolling, paused, complete, failed | |
| `TestRolloutPauseOnFailure` | `internal/component/plugin/server/rollout_test.go` | Automatic pause when ACK failure exceeds threshold | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-device-register` | `test/managed/fleet-device-register.ci` | Device connects, appears in registry | |
| `fleet-show-devices` | `test/managed/fleet-show-devices.ci` | `show fleet devices` lists connected devices | |
| `fleet-template-fetch` | `test/managed/fleet-template-fetch.ci` | Group member receives rendered config | |
| `fleet-audit-log` | `test/managed/fleet-audit-log.ci` | Config push/ack recorded in audit trail | |
| `fleet-inventory-report` | `test/managed/fleet-inventory-report.ci` | Device reports inventory, hub stores and shows | |
| `fleet-staged-rollout` | `test/managed/fleet-staged-rollout.ci` | Rollout progresses through group, pauses on failure | |

## Files to Modify

This umbrella owns the release split and the child acceptance map. Every product change, source path, schema change and functional test belongs to the corresponding child in **Child Specs**. Implementation proceeds from that child's current files list and architecture decisions.

## Files to Create

None directly. The registry, reports, history, drift repair, alarm model, templates, audit history, rollouts and relay are separately reviewable child changes.

## Design presentation

### What changes

The hub uses the managed connection each declared device opens. The first release exposes the registry, reports, metric history, alarm list, alarm causes and drift repair through the shared command registry. Templates, fleet audit, staged rollouts, command relay and the fleet pages have post-release owners. Closed phases 6 and 7 supply the baseline, local hold and operator resolution today.

### Before and after

A device whose hub connection drops remains identifiable by its last contact. A locally edited device that reconnects holds its configuration until the operator adopts or reverts it. A device in an unstarted rollout batch still fetches its previously admitted version. First-release operators inspect these first-release facts through commands. The post-release pages consume the same backend facts.

### Alternatives set aside

A reverse SSH tunnel would create another transport and identity path. A hub-side HTTP proxy would import another daemon’s session into the hub. A separate database would duplicate the persistence lifetime already owned by ZeFS. Each child extends the managed transport, dispatcher or existing store as appropriate.

### Documents departed from

`docs/architecture/fleet-config.md`, Non-Goals, describes sending full configurations on changes. Fleet-10 owns the explicit departure: use verified line diffs where economical and a full transfer when a diff is unavailable or cannot be verified. Local edits still hold until the operator resolves them. The architecture page changes with that child.

### Choices for Thomas

The choices in the Task are settled. No further operator decision is needed for this design.

## Implementation Steps

### Implementation Phases

Each phase corresponds to a child spec. Phases are ordered by dependency. A
child spec's own Implementation Steps, Files and test tables are the
authority for its phase; where this list names a file or a test the child no
longer plans, the child wins.

1. **Phase: Device Registry (fleet-1, `spec-fleet-1-device-registry`, closed 2026-11-25)** -- persistent device record, `show fleet devices` and `show fleet device name <name>`, labels in config, the reload apply, the connection bound (the estate part of the fleet interface and the `/fleet` pages moved to `fleet-ui-2`, after the first release)
   - Tests: `TestRegistryFirstConnectRecordsConnected` and the other `managed_registry_test.go` tests, `fleet-devices-connect.ci`, `fleet-device-detail.ci`, `fleet-devices-restart.ci`, `fleet-devices-reload-add.ci`, `fleet-devices-label.ci`
   - Files: `managed_registry.go`, `managed_apply.go`, `managed.go`, `managed_serve.go`, `internal/component/managed/cmd_devices.go`, `cmd/ze/hub/main_reload.go`
   - Verify: devices appear in the registry on connect, survive a hub restart as offline, and are queryable on every command surface

2. **Phase: Config Templates (fleet-2, `plan/post-release/features/spec-fleet-2-config-templates.md`)** -- template rendering, group assignment
   - Tests: `TestConfigTemplateRender`, `fleet-template-fetch.ci`
   - Files: `template.go`, `managed.go` (shared desired-config reader for fetch and notification)
   - Verify: group members receive rendered config, template change triggers config-changed

3. **Phase: Audit Trail (fleet-3, `plan/post-release/features/spec-fleet-3-audit-trail.md`)** -- centralized fleet operation logging
   - Tests: `TestAuditLogAppend`, `fleet-audit-log.ci`
   - Files: `audit.go`, `managed.go` (hook audit on connect/config-ack/disconnect)
   - Verify: audit entries queryable through commands on every command surface; the audit web view is `fleet-ui-6`'s

4. **Phase: Inventory + Health (fleet-4, `spec-fleet-4-inventory-health`, closed 2026-11-26)** -- device reporting, hub aggregation
   - Tests: `TestInventoryReportHandler`, `TestHealthReportHandler`, `fleet-inventory-report.ci`
   - Files: `fleet_report.go`, `report.go`, `client.go` (add reporting to client loop)
   - Verify: device inventory and health answered on the hub by `show fleet device inventory name <name>`, `show fleet device health name <name>` and `show fleet health` on every command surface; the web views are `fleet-ui-2`'s, `fleet-ui-3`'s and `fleet-ui-4`'s, after the first release

5. **Phase: Staged Rollout (fleet-5, `plan/post-release/features/spec-fleet-5-staged-rollout.md`)** -- orchestrated config push with failure gates
   - Tests: `TestRolloutStateMachine`, `TestRolloutPauseOnFailure`, `fleet-staged-rollout.ci`
   - Files: `rollout.go`, `managed.go` (rollout-aware config-changed dispatch)
   - Verify: rollout progresses through group, pauses on failure threshold

6. **Phase: Config Freeze (fleet-6, closed 2026-11-19)** -- persisted baseline, reconnect hold, `request fleet disable` and `enable`, `show fleet status`
   - Design: `docs/architecture/fleet-config.md`, "The baseline and the hold" and "The freeze"

7. **Phase: Reconnect Resolution (fleet-7, closed 2026-11-19)** -- the `config-held` up-verb, the hub's held record, adopt or revert of a diverged device
   - Design: `docs/architecture/fleet-config.md`, "The baseline and the hold" ("The hub's copy of a hold")

8. **Phase: Metric History (fleet-8, `spec-fleet-8-metric-history`, closed 2026-11-27)** -- devices report a fixed 15-figure sample set; the hub keeps a bounded, persisted dual-resolution history and answers series, latest-value, ranking, reduction, and storage queries
   - Design: `docs/architecture/fleet-config.md`, "Device metric sampling and hub-side history"
   - Depends: phase 4
   - Tests and files: settled by the child's design

9. **Phase: Device Command Relay (fleet-9, `plan/post-release/features/spec-fleet-9-device-command-relay.md`)** -- the hub relays an operator's command to a device over the managed channel, under a profile the device grants
   - Depends: phase 1
   - Tests and files: settled by the child's design

10. **Phase: Drift Hash Chain (fleet-10, `plan/release/features/spec-fleet-10-drift-hash-chain.md`)** -- hub and client fold every applied hub change into a chain, compare its head on every reply, resync a client that holds no local edit, and open a Configuration resync failed alarm when repair fails
   - Depends: phases 1 and 7, `spec-fleet-ui-5-alarm-list` (closed 2026-11-27; `docs/architecture/fleet-alarms.md`), `plan/release/features/spec-fleet-ui-8-alarm-causes.md`
   - Tests and files: settled by the child's design

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every child spec AC-N has implementation with file:line |
| Correctness | Existing config-fetch protocol unchanged; templates transparent to client |
| Naming | Fleet YANG uses kebab-case; RPC verbs use kebab-case; CLI uses `show fleet` prefix |
| Data flow | Device to hub to ZeFS to every command surface; no bypassed layers |
| Rule: wiring-completeness | Every new exported symbol called from cmd/ze/hub/main.go or a registered command handler |
| Rule: no-partial-completion | No child spec marked done without wiring test passing |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Device registry with persistence | `grep -rn DeviceRegistry internal/component/plugin/server/` |
| `show fleet devices` command | `grep -rn 'fleet.*devices' internal/component/managed/` |
| Fleet commands on every command surface | `TestEveryCommandOnEverySurface` (`cmd/ze/hub/surface_parity_test.go`) lists and serves them |
| Config template rendering | `grep -rn ConfigTemplate internal/component/plugin/server/` |
| Audit log with query API | `grep -rn AuditLog internal/component/plugin/server/` |
| Inventory + health reporting | `grep -rn inventory-report pkg/fleet/` |
| Staged rollout state machine | `grep -rn Rollout internal/component/plugin/server/` |
| 6 functional tests | `ls test/managed/fleet-*.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Inventory and health report payloads bounded (max size, field count) |
| Label injection | Label keys/values validated (alphanumeric + limited punctuation, no control chars) |
| Template injection | Template variables use safe substitution (no eval, no shell expansion) |
| Audit integrity | Audit log is append-only; no delete or modify API |
| Auth scope | Each query and operation is checked against its caller’s authorized scope. The device additionally enforces its own grant for relayed commands. |
| DoS | Rate-limit inventory/health reports per client to prevent blob growth |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Design
- [ ] No premature abstraction (each child spec is a concrete deliverable)
- [ ] No speculative features (every child addresses a gap identified in analysis)
- [ ] Single responsibility per child spec
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (children are independent where possible)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated across child specs
- [ ] Wiring Test table complete
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
