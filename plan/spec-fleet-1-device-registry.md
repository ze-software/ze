# Spec: fleet-1 -- Device Registry and Fleet Dashboard

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-05-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `docs/architecture/fleet-config.md` -- existing fleet config architecture
4. `internal/component/plugin/server/managed.go` -- ManagedConfigService (extend this)
5. `spec-fleet-0-umbrella.md` -- umbrella design decisions

## Task

Build a persistent hub-side device registry that tracks all known managed devices with
their metadata: name, config version, health status, last-seen timestamp, online/offline
state, and operator-assigned labels (flat key=value pairs). Expose the registry through
CLI commands (`show fleet devices`, `show fleet device detail <name>`) and a web dashboard page.

This is the foundational fleet spec. All other fleet specs (templates, audit, inventory,
rollout) depend on the device registry for device identity and grouping.

Currently `ManagedConfigService.connected` is a `map[string]struct{}` in memory. It tracks
which clients are connected right now but stores nothing about them and forgets everything
on restart. This spec replaces that with a persistent `DeviceRegistry` backed by ZeFS blob
storage, and adds a YANG `fleet {}` config container for operator-managed device metadata
(labels, groups).

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| Storage | ZeFS blob, keyed by device name. No external database |
| Labels | Flat key=value pairs. Validated: alphanumeric keys, printable values, bounded count |
| State tracking | Online/offline derived from connection state. Last-seen persisted. Config version from last config-fetch ACK |
| Diverged state (fleet-6/7) | A device may also be in a `diverged` state: it reconnected with a config that differs from its baseline (an emergency local edit), detected and held by `fleet-6`. The registry records it so the dashboard can warn and offer the resolution diff; set by `fleet-6`, cleared on resolve by `fleet-7`. Named `diverged`, not `conflict`, to avoid collision with the editor's existing `Conflict` types |
| YANG | `fleet { device <name> { label <key> <value>; group <name>; } }` under hub config |
| CLI | `show fleet devices [--group <g>] [--label <k>=<v>]`, `show fleet device detail <name>` |
| Web | New page `/fleet` with device table, status indicators, filter by group/label |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- existing fleet config architecture
  -> Constraint: preserve existing protocol; registry is additive
- [ ] `docs/architecture/core-design.md` -- component lifecycle, registration pattern
  -> Constraint: fleet component follows registration pattern
- [ ] `internal/component/plugin/server/managed.go` -- ManagedConfigService
  -> Decision: extend with DeviceRegistry, not replace

**Key insights:**
- RegisterClient/UnregisterClient are the natural hooks for registry updates
- HandleConfigFetch already receives clientName; config version available from response
- Web UI uses HTMX + SSE for live updates; fleet dashboard should follow same pattern

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed.go` -- ManagedConfigService with map[string]struct{} for connected clients
  -> Constraint: RegisterClient/UnregisterClient signatures must remain compatible
- [ ] `pkg/fleet/envelope.go` -- config-fetch response includes version hash
  -> Decision: capture version from HandleConfigFetch response for registry
- [ ] `internal/component/web/page_system.go` -- CollectFleetPeers for web peer selector
  -> Constraint: preserve existing fleet peer selector alongside new dashboard
- [ ] `internal/component/web/handler_workbench.go` -- workbench topbar wires fleet peers
  -> Constraint: fleet dashboard is a new page, not a replacement for workbench

**Behavior to preserve:**
- ManagedConfigService API (RegisterClient, UnregisterClient, HandleConfigFetch, BuildConfigChanged)
- Existing web fleet peer selector
- Client protocol unchanged (no new client-side code)

**Behavior to change:**
- RegisterClient persists device record to ZeFS blob on first connect
- UnregisterClient updates last-seen and sets offline state
- HandleConfigFetch records config version in device record
- New YANG `fleet {}` container for device labels and groups
- New CLI commands under `show fleet`
- New web page at `/fleet`

## Data Flow (MANDATORY)

### Entry Point
- Device connects to hub (existing TLS auth path)
- Operator configures `fleet { device edge-01 { label role edge; group region-west; } }`
- Operator runs `show fleet devices`

### Transformation Path
1. Device authenticates via existing `#0 auth` path
2. RegisterClient extended: persist device record to ZeFS blob (name, first-seen, online=true)
3. HandleConfigFetch extended: update config-version in device record
4. UnregisterClient extended: update last-seen, set online=false
5. YANG fleet config parsed at startup/reload: labels and groups loaded into registry
6. CLI/web queries registry for device list with optional filters

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Auth to Registry | RegisterClient call after successful auth | [ ] |
| Registry to ZeFS | Blob read/write for device records | [ ] |
| Registry to CLI/Web | Query API for device list and detail | [ ] |
| YANG config to Registry | Config tree extraction at startup/reload | [ ] |

### Integration Points
- `ManagedConfigService` -- extend with DeviceRegistry field
- `PluginAcceptor.handleConn` -- already calls RegisterClient (no change needed)
- EventBus -- emit `fleet.device.online`, `fleet.device.offline` events for SSE
- YANG schema -- `fleet {}` container under `plugin {}` or top-level
- CLI -- `show fleet` command family
- Web -- `/fleet` page

### Architectural Verification
- [ ] No bypassed layers (registry uses existing auth path)
- [ ] No unintended coupling (standalone devices ignore fleet config)
- [ ] No duplicated functionality (extends ManagedConfigService)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Device connects and authenticates | -> | DeviceRegistry.Register persists record | `test/managed/fleet-device-register.ci` |
| Device disconnects | -> | DeviceRegistry updates last-seen, sets offline | `test/managed/fleet-device-register.ci` |
| `show fleet devices` CLI | -> | Registry query returns device list | `test/managed/fleet-show-devices.ci` |
| Fleet config with labels | -> | Labels stored in registry, filterable | `test/managed/fleet-show-devices.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Device connects to hub for the first time | Device record created in ZeFS blob with name, first-seen, online=true |
| AC-2 | Device disconnects | Device record updated: online=false, last-seen=now |
| AC-3 | Device reconnects | Device record updated: online=true, last-seen=now. First-seen unchanged |
| AC-4 | Device completes config-fetch | Device record updated with config version hash |
| AC-5 | Hub restarts | Device registry loaded from ZeFS blob; all devices show offline until they reconnect |
| AC-6 | `show fleet devices` with no flags | Table of all registered devices: name, status, config-version, last-seen |
| AC-7 | `show fleet devices --group region-west` | Only devices in group region-west shown |
| AC-8 | `show fleet devices --label role=edge` | Only devices with label role=edge shown |
| AC-9 | `show fleet device detail edge-01` | Detail view: name, status, config-version, first-seen, last-seen, labels, groups |
| AC-10 | Fleet YANG config with `device edge-01 { label role edge; group region-west; }` | Labels and group stored in registry |
| AC-11 | Web `/fleet` page | Table of all devices with status, version, last-seen. Live SSE updates |
| AC-12 | Standalone hub (no managed clients configured) | Fleet registry empty, `show fleet devices` shows empty table, no errors |
| AC-13 | Device not declared in `fleet {}` config connects | Device still registered (auto-discovered), but has no labels or groups |
| AC-14 | Device reconnects in a `diverged` state (set by fleet-6) | Registry stores and exposes the `diverged` state; `show fleet devices` and the web dashboard flag it distinctly from online/offline |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDeviceRegistryRegister` | `internal/component/plugin/server/registry_test.go` | Register creates device record | |
| `TestDeviceRegistryUnregister` | `internal/component/plugin/server/registry_test.go` | Unregister sets offline, updates last-seen | |
| `TestDeviceRegistryPersist` | `internal/component/plugin/server/registry_test.go` | Records survive registry reload from blob | |
| `TestDeviceRegistryLabels` | `internal/component/plugin/server/registry_test.go` | Label assignment and query filtering | |
| `TestDeviceRegistryGroups` | `internal/component/plugin/server/registry_test.go` | Group assignment and query filtering | |
| `TestDeviceRegistryConfigVersion` | `internal/component/plugin/server/registry_test.go` | Config version updated on fetch | |
| `TestDeviceRegistryDuplicate` | `internal/component/plugin/server/registry_test.go` | Duplicate client rejected (existing behavior preserved) | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-device-register` | `test/managed/fleet-device-register.ci` | Device connects, appears in registry, disconnects, shows offline | |
| `fleet-show-devices` | `test/managed/fleet-show-devices.ci` | Multiple devices connected, `show fleet devices` lists all with correct state | |

## Files to Modify
- `internal/component/plugin/server/managed.go` -- extend ManagedConfigService with DeviceRegistry
- `cmd/ze/hub/main.go` -- wire DeviceRegistry at startup, pass to ManagedConfigService
- `internal/component/plugin/yang/ze-plugin-conf.yang` -- add fleet container (or new YANG module)

## Files to Create
- `internal/component/plugin/server/registry.go` -- DeviceRegistry type and persistence
- `internal/component/plugin/server/registry_test.go` -- unit tests
- `internal/component/web/page_fleet.go` -- fleet dashboard web page
- `internal/component/web/templates/page/fleet.html` -- fleet dashboard template
- `test/managed/fleet-device-register.ci` -- functional test
- `test/managed/fleet-show-devices.ci` -- functional test

## Implementation Steps

### Implementation Phases

1. **Phase: DeviceRegistry type** -- core registry with CRUD, persistence, label/group support
   - Tests: `TestDeviceRegistryRegister`, `TestDeviceRegistryUnregister`, `TestDeviceRegistryPersist`, `TestDeviceRegistryLabels`, `TestDeviceRegistryGroups`, `TestDeviceRegistryConfigVersion`, `TestDeviceRegistryDuplicate`
   - Files: `registry.go`, `registry_test.go`
   - Verify: unit tests pass

2. **Phase: Wire into ManagedConfigService** -- extend RegisterClient/UnregisterClient/HandleConfigFetch
   - Tests: existing managed tests still pass + `fleet-device-register.ci`
   - Files: `managed.go`, `cmd/ze/hub/main.go`
   - Verify: device records persist across hub restart

3. **Phase: YANG schema + CLI** -- fleet config container, `show fleet` commands
   - Tests: `fleet-show-devices.ci`
   - Files: YANG schema, CLI command registration
   - Verify: CLI shows device list with labels and filters

4. **Phase: Web dashboard** -- `/fleet` page with HTMX table and SSE updates
   - Tests: manual browser verification
   - Files: `page_fleet.go`, `fleet.html`
   - Verify: page renders, SSE updates on connect/disconnect

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N have implementation with file:line |
| Correctness | Existing RegisterClient/UnregisterClient behavior preserved |
| Naming | YANG uses kebab-case, CLI uses `show fleet` prefix, blob keys follow namespace convention |
| Data flow | Auth -> RegisterClient -> blob persist -> CLI/web query |
| Rule: wiring-completeness | DeviceRegistry called from cmd/ze/hub/main.go |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| DeviceRegistry type | `grep -rn DeviceRegistry internal/component/plugin/server/` |
| Persistence in ZeFS | `grep -rn blob internal/component/plugin/server/registry.go` |
| `show fleet devices` CLI | `grep -rn 'fleet' internal/component/cli/` |
| Web fleet page | `ls internal/component/web/page_fleet.go` |
| Functional tests | `ls test/managed/fleet-device-register.ci test/managed/fleet-show-devices.ci` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Label keys: alphanumeric + hyphen/underscore, max 64 chars. Values: printable, max 256 chars. Max 32 labels per device |
| Auth scope | `show fleet` requires read authorization; fleet config changes require admin |
| Blob key injection | Device names are already validated by auth; label keys need validation |

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-standard-test` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Architecture docs updated

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
