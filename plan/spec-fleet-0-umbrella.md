# Spec: fleet-0 -- Fleet Management (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/fleet-config.md` -- existing fleet config architecture
4. `internal/component/plugin/server/managed.go` -- hub-side managed config service
5. `internal/component/managed/` -- client-side managed lifecycle
6. `pkg/fleet/` -- shared RPC types and version hashing
7. Child specs: `spec-fleet-1-*` through `spec-fleet-5-*`

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
| Fleet CLI + web dashboard | `show fleet devices`, `show fleet device detail <name>`, web page listing fleet state |
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

All seven children moved to `plan/future/` on 2026-08-29: the fleet set is new
capability, not a defect in the shipped product, so it does not hold the first
release. This umbrella stays in `plan/`.

| Phase | Spec | Scope | Depends |
|-------|------|-------|---------|
| 1 | `plan/future/spec-fleet-1-device-registry.md` | Hub-side persistent device registry. `show fleet` CLI. Web fleet dashboard. Device metadata (name, config version, health, last-seen, labels). YANG schema for fleet config | - |
| 2 | `plan/future/spec-fleet-2-config-templates.md` | Named config templates with variable substitution. Group-based assignment. Template rendering at config-fetch time. YANG schema for templates and groups | fleet-1 |
| 3 | `plan/future/spec-fleet-3-audit-trail.md` | Centralized audit log for fleet operations: config push/ack/reject, device connect/disconnect, template changes. CLI and web views. Structured log entries in ZeFS | fleet-1 |
| 4 | `plan/future/spec-fleet-4-inventory-health.md` | New `inventory-report` and `health-report` RPC verbs. Devices push on connect and periodically. Hub stores per-device inventory and health. CLI and web aggregated views | fleet-1 |
| 5 | `plan/future/spec-fleet-5-staged-rollout.md` | Percentage-based config push targeting device groups. Rollout state machine (pending, rolling, paused, complete, failed). Automatic pause on ACK failure threshold. CLI rollout commands | fleet-2, fleet-4 |
| 6 | `plan/future/spec-fleet-6-config-freeze.md` | Freeze operator config edits (single-writer); hub-side connected-only guard; persisted baseline + reconnect hold (no silent stomp); `ze fleet disable/enable/status` with immediate sever | - |
| 7 | `plan/future/spec-fleet-7-config-reconnect-resolution.md` | Resolve a `diverged` device: `config-push` up-verb (existing TLS), commit-style diff (Local=hub, Remote=router), adopt/revert | fleet-6 |

Phases 2, 3, and 4 are independent of each other (all depend only on fleet-1) and can proceed in parallel. Phase 5 depends on templates (for group targeting) and health reporting (for rollout health gates). Phases 6 and 7 are the direction update below: 6 (freeze, persisted baseline, safe reconnect hold) depends on nothing new and is safe to land first; 7 (diverged-config resolution) depends on 6 and surfaces through the fleet-1 dashboard.

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
- Web fleet peer selector (config-declared peer URLs) preserved alongside new fleet dashboard

**Behavior to change:**
- Hub-side ManagedConfigService extended with persistent device registry
- New YANG `fleet {}` container for fleet configuration (device labels, templates, groups)
- New RPC verbs for inventory and health reporting (fleet-4)
- Config fetch extended to support template rendering (fleet-2)
- New CLI commands under `show fleet` and `fleet rollout` (fleet-1, fleet-5)
- New web pages for fleet dashboard (fleet-1)

## Data Flow (MANDATORY)

### Entry Point
- Device connects to hub via TLS (existing path)
- Hub-initiated config-changed notifications (existing path)
- New: device-initiated inventory-report and health-report RPCs (fleet-4)
- New: operator CLI/web commands for fleet management

### Transformation Path
1. Device connects, authenticates (existing)
2. Hub registers device in persistent registry with metadata (fleet-1, extends existing RegisterClient)
3. Device requests config via config-fetch (existing)
4. Hub renders config from template if group-assigned (fleet-2, intercepts existing HandleConfigFetch)
5. Device reports inventory and health (fleet-4, new RPC handling)
6. Hub stores device state, exposes via CLI/web/API (fleet-1)
7. Operator initiates staged rollout (fleet-5), hub orchestrates notifications

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Device to Hub | TLS + MuxConn RPC (existing transport) | [ ] |
| Hub to ZeFS | Blob read/write for device registry, audit log, templates | [ ] |
| Hub to CLI/Web | Event bus + query API for fleet state | [ ] |
| Template to Config | Hub-side rendering at config-fetch time | [ ] |

### Integration Points
- `ManagedConfigService` -- extend with registry, template lookup, audit logging
- `PluginAcceptor.handleConn` -- hook device registration on auth success
- EventBus -- fleet events for web SSE and internal consumers
- YANG schema -- `fleet {}` container alongside existing `plugin { hub {} }`
- CLI command tree -- `show fleet`, `fleet rollout`
- Web pages -- fleet dashboard, device detail, rollout status

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
| AC-6 | Device reports degraded health | Fleet dashboard shows device as degraded; `show fleet health` aggregates fleet-wide status |
| AC-7 | Staged rollout at 20% with failure | Rollout pauses automatically; `show fleet rollout` shows pause reason and affected devices |
| AC-8 | Standalone device (not managed) | Zero fleet overhead; no fleet YANG required; existing behavior unchanged |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Device connects to hub with per-client auth | -> | DeviceRegistry.Register persists device record | `test/managed/fleet-device-register.ci` |
| `show fleet devices` CLI command | -> | Fleet query returns all registered devices | `test/managed/fleet-show-devices.ci` |
| Config template assigned to device group | -> | HandleConfigFetch renders template for group member | `test/managed/fleet-template-fetch.ci` |
| Device sends inventory-report RPC | -> | Hub stores inventory, queryable via CLI | `test/managed/fleet-inventory-report.ci` |
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
- `internal/component/web/page_system.go` -- fleet dashboard web page
- `internal/component/plugin/yang/ze-plugin-conf.yang` -- fleet YANG container

## Files to Create
- `internal/component/plugin/server/registry.go` -- DeviceRegistry: persistent device state
- `internal/component/plugin/server/template.go` -- config template rendering
- `internal/component/plugin/server/audit.go` -- fleet audit log
- `internal/component/plugin/server/rollout.go` -- staged rollout state machine
- `internal/component/plugin/server/fleet_report.go` -- inventory and health report handlers
- `pkg/fleet/report.go` -- inventory-report and health-report envelope types
- `internal/component/web/page_fleet.go` -- fleet dashboard web page
- `test/managed/fleet-*.ci` -- functional tests (6 files)

## Implementation Steps

### Implementation Phases

Each phase corresponds to a child spec. Phases are ordered by dependency.

1. **Phase: Device Registry (fleet-1)** -- persistent device state, fleet CLI, web dashboard
   - Tests: `TestDeviceRegistryRegister`, `TestDeviceRegistryLabels`, `fleet-device-register.ci`, `fleet-show-devices.ci`
   - Files: `registry.go`, `managed.go`, `cmd/ze/hub/main.go`, `page_fleet.go`
   - Verify: devices appear in registry on connect, queryable via CLI and web

2. **Phase: Config Templates (fleet-2)** -- template rendering, group assignment
   - Tests: `TestConfigTemplateRender`, `fleet-template-fetch.ci`
   - Files: `template.go`, `managed.go` (HandleConfigFetch intercept)
   - Verify: group members receive rendered config, template change triggers config-changed

3. **Phase: Audit Trail (fleet-3)** -- centralized fleet operation logging
   - Tests: `TestAuditLogAppend`, `fleet-audit-log.ci`
   - Files: `audit.go`, `managed.go` (hook audit on connect/config-ack/disconnect)
   - Verify: audit entries queryable via CLI and web

4. **Phase: Inventory + Health (fleet-4)** -- device reporting, hub aggregation
   - Tests: `TestInventoryReportHandler`, `TestHealthReportHandler`, `fleet-inventory-report.ci`
   - Files: `fleet_report.go`, `report.go`, `client.go` (add reporting to client loop)
   - Verify: device inventory and health visible on hub via CLI and web

5. **Phase: Staged Rollout (fleet-5)** -- orchestrated config push with failure gates
   - Tests: `TestRolloutStateMachine`, `TestRolloutPauseOnFailure`, `fleet-staged-rollout.ci`
   - Files: `rollout.go`, `managed.go` (rollout-aware config-changed dispatch)
   - Verify: rollout progresses through group, pauses on failure threshold

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every child spec AC-N has implementation with file:line |
| Correctness | Existing config-fetch protocol unchanged; templates transparent to client |
| Naming | Fleet YANG uses kebab-case; RPC verbs use kebab-case; CLI uses `show fleet` prefix |
| Data flow | Device to hub to ZeFS to CLI/web; no bypassed layers |
| Rule: wiring-completeness | Every new exported symbol called from cmd/ze/hub/main.go or web handlers |
| Rule: no-partial-completion | No child spec marked done without wiring test passing |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Device registry with persistence | `grep -rn DeviceRegistry internal/component/plugin/server/` |
| `show fleet devices` CLI command | `grep -rn 'fleet.*devices' internal/component/cli/` |
| Fleet web dashboard | `ls internal/component/web/page_fleet.go` |
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
