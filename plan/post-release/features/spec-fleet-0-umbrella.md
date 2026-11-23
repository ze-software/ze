# Spec: fleet-0 -- Fleet Management (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P3 |
| Release reason | Adds a capability absent from the supported release; existing supported behavior and its obligations remain required. |
| Depends | - |
| Phase | - |
| Updated | 2026-11-23 |

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

Extend Ze's fleet management from a config-distribution mechanism into a usable fleet
operations platform. The foundation is solid: TLS transport, per-client auth, config
fetch/push, transactional commits with rollback, health checks, rich hardware inventory,
and Prometheus metrics all exist at the single-device level. The gap is entirely hub-side:
collecting, storing, querying, and acting on fleet-wide state.

An operator managing 10-1000 Ze instances currently cannot answer basic fleet questions:
"which devices are online?", "what config version is each running?", "which devices have
degraded health?". They cannot push config changes to groups of devices, audit who changed
what, or do staged rollouts. This spec set addresses those gaps incrementally, each child
spec building on the previous.

### Existing Foundation

| Capability | Package | Status |
|------------|---------|--------|
| TLS 1.3 transport, MuxConn RPC | `pkg/plugin/rpc/`, `internal/component/plugin/ipc/tls.go` | Implemented |
| Per-client token auth | `internal/component/plugin/ipc/tls.go` (AuthenticateWithLookup) | Implemented |
| Config fetch/push (1:1) | `pkg/fleet/`, `internal/component/managed/`, `internal/component/plugin/server/managed.go` | Implemented |
| Heartbeat + reconnect with backoff | `internal/component/managed/heartbeat.go`, `reconnect.go` | Implemented |
| Local ZeFS config cache | `internal/component/managed/handler.go` | Implemented |
| Connected client tracking (in-memory) | `ManagedConfigService.connected` map | Implemented |
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
| Config templates are config, not code | Template variables and group assignment declared in YANG schema under `fleet {}`. Operators manage them through the existing CLI editor and web config editor |
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
| PKI/certificate lifecycle | Separate concern; current ephemeral self-signed certs + token auth works for the target scale. Future spec if needed |
| Multi-hub replication | Explicitly a non-goal (learned 444). HA via client cached config |
| Zero-touch provisioning (ZTP) | Requires DHCP option integration and unsigned bootstrap. Separate spec |
| Firmware distribution | Covered by existing `spec-cpe-5-firmware-update.md` (version check) and `ze appliance push` (OTA delivery) |
| Maintenance windows | Requires alerting integration not yet in Ze. Future spec |
| Fleet-wide rollback | Achievable as staged rollout with 100% targeting + previous config version. No separate mechanism needed |
| External metrics aggregation | Operators already use Prometheus. Fleet provides the scrape target list, not a replacement metrics store |
| Web per-device command routing | Hub proxying HTTP to a selected device over the persistent channel (for NAT'd CPE). High value but a separate spec; today's web fleet selector is UI-only links to each device's own URL |
| Dynamic enrollment + device identity | Runtime device approval and mTLS / cert-bound identity replacing the pre-declared shared secret (extends the PKI/cert-lifecycle row). Deferred; the shared secret is sufficient for the freeze/resolution step |
| Bidirectional config sync | Continuous two-way merge. Replaced for now by single-writer plus an operator-gated `config-push` at divergence (`fleet-7`); full bidirectional sync stays out of scope |

### Child Specs

The 2026-08-29 move treated the fleet set as optional capability. Fleet
management now ships in the first release (`ai/META-DECISIONS.md`, "Goal and
scope"): the device registry (phase 1), device health reports (phase 4),
metric history (phase 8) and configuration drift detection with repair by the
hub (phase 10) live in `plan/release/features/`, beside the fleet interface's
alarm list (`plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md`)
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
| 1 | `plan/release/features/spec-fleet-1-device-registry.md` | Hub-side persistent device registry. `show fleet devices` and `show fleet device name <name>` on every command surface. Device metadata (name, config version, last-seen, labels; health is `fleet-4`'s). Labels on the hub's client entry. The fleet web pages are `fleet-ui-2`'s, after the first release | - |
| 2 | `plan/post-release/features/spec-fleet-2-config-templates.md` | Named config templates with variable substitution. Group-based assignment. Template rendering at config-fetch time. YANG schema for templates and groups | fleet-1 |
| 3 | `plan/post-release/features/spec-fleet-3-audit-trail.md` | Centralized audit log for fleet operations: config push/ack/reject, device connect/disconnect, template changes. Commands on every command surface; the audit web views are `fleet-ui-6`'s. Structured log entries in ZeFS | fleet-1 |
| 4 | `plan/release/features/spec-fleet-4-inventory-health.md` | New `inventory-report` and `health-report` RPC verbs. Devices push on connect and periodically. Hub stores per-device inventory and health. `show fleet device inventory <name>`, `show fleet device health <name>` and `show fleet health` on every command surface. The inventory and health web views are `fleet-ui-2`'s, `fleet-ui-3`'s and `fleet-ui-4`'s, after the first release | fleet-1 |
| 5 | `plan/post-release/features/spec-fleet-5-staged-rollout.md` | Percentage-based config push targeting device groups. Rollout state machine (pending, rolling, paused, complete, failed). Automatic pause on ACK failure threshold. CLI rollout commands | fleet-2, fleet-4 |
| 6 | `spec-fleet-6-config-freeze` (closed 2026-11-19; design in `docs/architecture/fleet-config.md`, "The baseline and the hold" and "The freeze") | Persisted baseline and reconnect hold (no silent stomp); `request fleet disable` and `enable` with immediate sever, `show fleet status`; best-effort freeze of operator edits on a managed device; declared-client guard on every hub writer. See the Direction Update of 2026-11-19 | - |
| 7 | `spec-fleet-7-config-reconnect-resolution` | Resolve a `diverged` device: an up-verb on the existing TLS channel, a diff at the hub (Local=hub, Remote=router), adopt or revert, and its own per-client record of the pushed config. Closed 2026-11-19; design in `docs/architecture/fleet-config.md`, "The baseline and the hold" ("The hub's copy of a hold") | fleet-6 |

Phases 2, 3, and 4 are independent of each other (all depend only on fleet-1) and can proceed in parallel. Phase 5 depends on templates (for group targeting) and health reporting (for rollout health gates). Phases 6 and 7 are the direction update below: 6 (freeze, persisted baseline, safe reconnect hold) depends on nothing new and is safe to land first; 7 (diverged-config resolution) depends on 6 and surfaces through `show fleet divergence` and the `diverged` field of `fleet-1`'s `show fleet devices` (`fleet-1` AC-13); its web banner and diff are `fleet-ui-6`'s, after the first release.

### Direction Update (2026-06-26): Single-Writer Config, Emergency Disable, Reconnect Resolution

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
| 8 | `plan/release/features/spec-fleet-8-metric-history.md` | Devices report a fixed sample set; the hub keeps a bounded, persisted history and answers series, latest-value and ranking queries | fleet-4 |
| 9 | `plan/post-release/features/spec-fleet-9-device-command-relay.md` | The hub relays an operator's command to a device over the managed channel, under a profile the device grants, with streaming, deadline, cancellation and audit on both sides | fleet-1 |
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
| Device registry in ZeFS blob, not SQL | Ze has no SQL dependency. ZeFS blob is the universal storage. Device count target (hundreds, not millions) fits blob well |
| Labels are flat key=value pairs | Simple, proven (Kubernetes, Prometheus). No hierarchical grouping needed at this scale |
| Config templates rendered hub-side at fetch time | Client gets final config, never sees templates. Simplifies client (unchanged). Template changes trigger config-changed notifications for affected devices |
| Audit log is append-only blob entries | No rotation needed at fleet scale (hundreds of devices, ~KB per event). External log shipping is a future concern |
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
- [ ] `docs/architecture/hub-architecture.md` - the standalone orchestrator in `internal/component/hub/`
- [ ] `docs/architecture/web-workbench-pages.md` - the workbench shell and its reusable domain-page components
- [ ] `docs/architecture/fleet-config.md` -- existing fleet config architecture (all 17 ACs implemented)
  -> Decision: extend hub, not new server. Config-as-identity. Two-phase config change
  -> Constraint: preserve existing protocol and architecture
- [ ] `docs/architecture/core-design.md` -- component lifecycle, event bus, registration pattern
  -> Constraint: fleet component follows registration pattern; fleet events use EventBus
- [ ] `internal/component/plugin/server/managed.go` -- hub-side ManagedConfigService
  -> Decision: extend ManagedConfigService with persistent registry, not parallel service

**Key insights:**
- ManagedConfigService.connected is a `map[string]struct{}` (memory-only, no metadata)
- Four RPC verbs on `#id verb [json]\n` framing; new fleet RPCs follow same pattern
- Client is self-contained; fleet-1 through fleet-3 require zero client changes
- ZeFS blob is the universal storage for both hub and client side

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` -- hub-side managed config service: ConfigReader, ManagedConfigService with RegisterClient/UnregisterClient, HandleConfigFetch, BuildConfigChanged. Connected clients tracked in `map[string]struct{}` (memory-only, no persistence, no metadata)
  -> Constraint: ManagedConfigService is the natural anchor for fleet state. Extend it, don't replace it
- [ ] `pkg/fleet/envelope.go` -- four RPC verbs: config-fetch, config-changed, config-ack, ping. Line-oriented `#id verb [json]\n` framing
  -> Constraint: new fleet RPCs must follow the same framing convention
- [ ] `internal/component/managed/client.go` -- RunManagedClient: TLS connect, auth, config fetch, heartbeat+notification loop. CheckManaged callback before each reconnect
  -> Constraint: client remains unchanged for fleet-1 through fleet-3. Fleet-4 adds reporting calls to the client loop
- [ ] `internal/core/health/` -- health registry: component checks (healthy/degraded/down), aggregated report, HTTP endpoint
  -> Decision: health-report RPC sends the same structured data as the HTTP endpoint
- [ ] `internal/component/host/inventory.go` -- hardware inventory: CPU, NIC, DMI, memory, thermal, storage, SMART. Cached detection with TTL
  -> Decision: inventory-report RPC sends a subset (no raw SMART data; summary only)
- [ ] `docs/architecture/fleet-config.md` -- full architecture doc for existing fleet config implementation
  -> Constraint: all fleet-0 children must preserve the existing protocol and architecture described here

**Behavior to preserve:**
- Existing config-fetch/config-changed/config-ack/ping protocol unchanged
- Per-client auth and config isolation unchanged
- Client cached config for partition resilience unchanged
- Appliance toolchain (ze appliance push/config-push) unchanged
- Web fleet peer selector (config-declared peer URLs) preserved alongside the `fleet-ui` pages

**Behavior to change:**
- Hub-side ManagedConfigService extended with persistent device registry
- New YANG `fleet {}` container for fleet configuration (device labels, templates, groups)
- New RPC verbs for inventory and health reporting (fleet-4)
- Config fetch extended to support template rendering (fleet-2)
- New commands under `show fleet` and `fleet rollout` (fleet-1, fleet-4, fleet-5), each registered once and reached on every command surface
- No web page in the first release: the fleet web pages are the `fleet-ui` set's, after it (`fleet-ui-2` the estate tree and device tables, `fleet-ui-3` the dashboards, `fleet-ui-4` the device tabs, `fleet-ui-9` the task, notification and alarm pages)

## Existing Mechanisms (MANDATORY)

Added 2026-11-18, when the direction update above touched this spec. The rows
restate the Existing Foundation table of the Task section against the code as
it stands today; where that table names an older symbol or path, this one is
current.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Managed channel, hub side | `internal/component/plugin/server/managed_serve.go`, `ManagedServer` | Accepts managed devices over TLS, authenticates each by its per-client secret, serves `config-fetch`, pushes `config-changed`, and holds the connected set in memory under a cap of 128 | Extends: the registry (`fleet-1`), reports (`fleet-4`, `fleet-8`) and the relay (`fleet-9`) are added on this server, not on a second one |
| Managed channel, device side | `internal/component/managed/client.go`, `RunManagedClient` | Connects, fetches and caches its configuration, answers hub requests in one loop, heartbeats and reconnects | Extends: reporting (`fleet-4`, `fleet-8`), the freeze and baseline (`fleet-6`), the push-up verb (`fleet-7`) and relay workers (`fleet-9`) |
| Verbs and envelopes | `pkg/fleet/envelope.go` | Four verbs: `config-fetch`, `config-changed`, `config-ack`, `ping` | Extends: new verbs on the same framing; public API, changed with `docs/architecture/fleet-config.md` |
| RPC transport | `pkg/plugin/rpc`, `MuxConn` | Multiplexed requests with streamed answers, a 16 MiB frame bound and a write deadline | Extends: used as it is by every new verb |
| Hub-held device configuration | Store key `file/active/client-<name>.conf`, read by `ManagedServer` | One configuration per device, served on fetch | Extends: templates (`fleet-2`) render into it; the audit trail (`fleet-3`) records changes to it |
| Health registry | `internal/core/health` | Per-component state on each device, shown by `show health` | Extends: `fleet-4` reports it to the hub |
| Hardware inventory | `internal/component/host` | CPU, memory, interfaces, storage and platform facts on each device | Extends: `fleet-4` reports a summary to the hub |
| Bounded time series | `internal/component/l2tp/cqm.go`, the sample ring | Fixed buckets with configured retention, for L2TP quality | Extends: `fleet-8` uses the same shape for the hub's history |
| Command dispatcher | `internal/component/plugin/dispatch.go`, `CommandDispatcher` | Runs a command for a caller with authorization and audit on every surface | Extends: `fleet-9` supplies one bound to a device; every `show fleet` command registers into the same registry |
| Config transactions and archive | `internal/component/config/transaction`, `internal/component/config/archive` | Verify, apply and rollback of a configuration; archived versions | Extends: a pushed configuration is applied through it, and rollouts (`fleet-5`) rely on its refusal |
| State store | `internal/core/statestore` with keys in `pkg/zefs/keys.go` | The daemon's one persistent store | Extends: the registry, audit log, rollout state and history are stored through it |
| Appliance push over SSH | `internal/appliance`, `cmd_push.go` and `cmd_config_push.go` | Software and configuration push from a build machine | Does not apply: an offline toolchain, kept as it is beside the runtime channel |
| Web selector of other devices | `internal/component/web/page_system.go`, `CollectFleetPeers` | Links to other devices' own addresses | Does not apply: it holds no state; the fleet web interface is `fleet-ui` |

## Data Flow (MANDATORY)

### Entry Point
- Device connects to hub via TLS (existing path)
- Hub-initiated config-changed notifications (existing path)
- New: device-initiated inventory-report and health-report RPCs (fleet-4)
- New: operator commands for fleet management, on every command surface

### Transformation Path
1. Device connects, authenticates (existing)
2. Hub registers device in persistent registry with metadata (fleet-1, extends existing RegisterClient)
3. Device requests config via config-fetch (existing)
4. Hub renders config from template if group-assigned (fleet-2, intercepts existing HandleConfigFetch)
5. Device reports inventory and health (fleet-4, new RPC handling)
6. Hub stores device state and answers it through commands on every command surface (fleet-1, fleet-4); the `fleet-ui` pages read the same commands after the first release
7. Operator initiates staged rollout (fleet-5), hub orchestrates notifications

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Device to Hub | TLS + MuxConn RPC (existing transport) | [ ] |
| Hub to ZeFS | Blob read/write for device registry, audit log, templates | [ ] |
| Hub to command surfaces | Builtin commands through the daemon dispatcher, the same answer on the CLI, SSH, MCP, REST, gRPC and the web command line | [ ] |
| Template to Config | Hub-side rendering at config-fetch time | [ ] |

### Integration Points
- `ManagedConfigService` -- extend with registry, template lookup, audit logging
- `PluginAcceptor.handleConn` -- hook device registration on auth success
- Live page updates -- none in the first release: `fleet-1` emits no EventBus fleet event, and the pages' live updates are the `EventSource` of `fleet-ui-9`, after it
- YANG schema -- `fleet {}` container alongside existing `plugin { hub {} }`
- Command registry -- `show fleet`, `fleet rollout`, registered once and reached on every command surface
- Web pages -- none in the first release: the estate tree and device tables are `fleet-ui-2`'s, the dashboard `fleet-ui-3`'s, device detail `fleet-ui-4`'s, rollout status `fleet-ui-6`'s, the task, notification and alarm pages `fleet-ui-9`'s, all after the first release

### Architectural Verification
- [ ] No bypassed layers (fleet uses existing TLS/auth/MuxConn path)
- [ ] No unintended coupling (fleet is additive; standalone devices unaffected)
- [ ] No duplicated functionality (extends ManagedConfigService, not parallel)
- [ ] Zero-copy preserved where applicable

## Acceptance Criteria (Umbrella-Level)

These are the top-level outcomes. Each child spec has its own detailed ACs.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Hub with 10+ managed clients connected | `show fleet devices` lists all with name, config version, health status, last-seen |
| AC-2 | Device disconnects and reconnects | Registry shows offline period, last-seen updates on reconnect |
| AC-3 | Config template assigned to a group | All devices in the group receive rendered config on next fetch |
| AC-4 | Operator pushes config change to a group | Audit trail records: who, when, which template, which devices, ack/reject per device |
| AC-5 | Device reports inventory to hub | `show fleet device inventory edge-01` shows CPU, NIC, memory, storage summary |
| AC-6 | Device reports degraded health | `show fleet device health <name>` shows the component degraded and `show fleet health` counts the device degraded, on every command surface (`fleet-4` AC-4, AC-7, AC-13). The dashboard's Degraded count moved to `fleet-ui-3` AC-8, after the first release |
| AC-7 | Staged rollout at 20% with failure | Rollout pauses automatically; `show fleet rollout` shows pause reason and affected devices |
| AC-8 | Standalone device (not managed) | Zero fleet overhead; no fleet YANG required; existing behavior unchanged |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Device connects to hub with per-client auth | -> | DeviceRegistry.Register persists device record | `test/managed/fleet-device-register.ci` |
| `show fleet devices` CLI command | -> | Fleet query returns all registered devices | `test/managed/fleet-show-devices.ci` |
| Config template assigned to device group | -> | HandleConfigFetch renders template for group member | `test/managed/fleet-template-fetch.ci` |
| Device sends inventory-report RPC | -> | Hub stores inventory, answered by `show fleet device inventory <name>` | `test/managed/fleet-inventory-report.ci` |
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
- `internal/component/plugin/server/managed.go` -- extend ManagedConfigService with persistent registry
- `pkg/fleet/envelope.go` -- add new RPC verb constants (inventory-report, health-report)
- `internal/component/managed/client.go` -- add inventory and health reporting to client loop (fleet-4)
- `cmd/ze/hub/main.go` -- wire fleet registry, audit, template, rollout at startup
- `internal/component/plugin/yang/ze-plugin-conf.yang` -- fleet YANG container

## Files to Create
- `internal/component/plugin/server/registry.go` -- DeviceRegistry: persistent device state
- `internal/component/plugin/server/template.go` -- config template rendering
- `internal/component/plugin/server/audit.go` -- fleet audit log
- `internal/component/plugin/server/rollout.go` -- staged rollout state machine
- `internal/component/plugin/server/fleet_report.go` -- inventory and health report handlers
- `pkg/fleet/report.go` -- inventory-report and health-report envelope types
- `test/managed/fleet-*.ci` -- functional tests (6 files)

No web file is planned here: the fleet pages are `internal/component/web/fleetui`'s, built by the `fleet-ui` set after the first release (`fleet-ui-2`, `fleet-ui-3`, `fleet-ui-4`, `fleet-ui-9`). `internal/component/web/page_system.go` keeps its peer selector unchanged.

## Implementation Steps

### Implementation Phases

Each phase corresponds to a child spec. Phases are ordered by dependency. A
child spec's own Implementation Steps, Files and test tables are the
authority for its phase; where this list names a file or a test the child no
longer plans, the child wins.

1. **Phase: Device Registry (fleet-1, `plan/release/features/spec-fleet-1-device-registry.md`, ready)** -- persistent device record, `show fleet devices` and `show fleet device name <name>`, labels in config, the reload apply, the connection bound (the estate part of the fleet interface and the `/fleet` pages moved to `fleet-ui-2`, after the first release)
   - Tests: `TestRegistryFirstConnectRecordsConnected` and the other `managed_registry_test.go` tests, `fleet-devices-connect.ci`, `fleet-device-detail.ci`, `fleet-devices-restart.ci`, `fleet-devices-reload-add.ci`, `fleet-devices-label.ci`
   - Files: `managed_registry.go`, `managed_apply.go`, `managed.go`, `managed_serve.go`, `internal/component/managed/cmd_devices.go`, `cmd/ze/hub/main_reload.go`
   - Verify: devices appear in the registry on connect, survive a hub restart as offline, and are queryable on every command surface

2. **Phase: Config Templates (fleet-2, `plan/post-release/features/spec-fleet-2-config-templates.md`, skeleton)** -- template rendering, group assignment
   - Tests: `TestConfigTemplateRender`, `fleet-template-fetch.ci`
   - Files: `template.go`, `managed.go` (HandleConfigFetch intercept)
   - Verify: group members receive rendered config, template change triggers config-changed

3. **Phase: Audit Trail (fleet-3, `plan/post-release/features/spec-fleet-3-audit-trail.md`, skeleton)** -- centralized fleet operation logging
   - Tests: `TestAuditLogAppend`, `fleet-audit-log.ci`
   - Files: `audit.go`, `managed.go` (hook audit on connect/config-ack/disconnect)
   - Verify: audit entries queryable through commands on every command surface; the audit web view is `fleet-ui-6`'s

4. **Phase: Inventory + Health (fleet-4, `plan/release/features/spec-fleet-4-inventory-health.md`, skeleton)** -- device reporting, hub aggregation
   - Tests: `TestInventoryReportHandler`, `TestHealthReportHandler`, `fleet-inventory-report.ci`
   - Files: `fleet_report.go`, `report.go`, `client.go` (add reporting to client loop)
   - Verify: device inventory and health answered on the hub by `show fleet device inventory <name>`, `show fleet device health <name>` and `show fleet health` on every command surface; the web views are `fleet-ui-2`'s, `fleet-ui-3`'s and `fleet-ui-4`'s, after the first release

5. **Phase: Staged Rollout (fleet-5, `plan/post-release/features/spec-fleet-5-staged-rollout.md`, skeleton)** -- orchestrated config push with failure gates
   - Tests: `TestRolloutStateMachine`, `TestRolloutPauseOnFailure`, `fleet-staged-rollout.ci`
   - Files: `rollout.go`, `managed.go` (rollout-aware config-changed dispatch)
   - Verify: rollout progresses through group, pauses on failure threshold

6. **Phase: Config Freeze (fleet-6, closed 2026-11-19)** -- persisted baseline, reconnect hold, `request fleet disable` and `enable`, `show fleet status`
   - Design: `docs/architecture/fleet-config.md`, "The baseline and the hold" and "The freeze"

7. **Phase: Reconnect Resolution (fleet-7, closed 2026-11-19)** -- the `config-held` up-verb, the hub's held record, adopt or revert of a diverged device
   - Design: `docs/architecture/fleet-config.md`, "The baseline and the hold" ("The hub's copy of a hold")

8. **Phase: Metric History (fleet-8, `plan/release/features/spec-fleet-8-metric-history.md`, skeleton)** -- devices report a fixed sample set; the hub keeps a bounded, persisted history and answers series, latest-value and ranking queries
   - Depends: phase 4
   - Tests and files: settled by the child's design

9. **Phase: Device Command Relay (fleet-9, `plan/post-release/features/spec-fleet-9-device-command-relay.md`, skeleton)** -- the hub relays an operator's command to a device over the managed channel, under a profile the device grants
   - Depends: phase 1
   - Tests and files: settled by the child's design

10. **Phase: Drift Hash Chain (fleet-10, `plan/release/features/spec-fleet-10-drift-hash-chain.md`, skeleton)** -- hub and client fold every applied hub change into a chain, compare its head on every reply, resync a client that holds no local edit, and open a Configuration resync failed alarm when repair fails
   - Depends: phases 1 and 7, `plan/release/features/spec-fleet-ui-5-tasks-alarms-notifications.md`, `plan/release/features/spec-fleet-ui-8-alarm-causes.md`
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
| Auth scope | Fleet CLI commands require admin authorization profile |
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
