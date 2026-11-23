# Spec: fleet-4 -- Inventory and Health Reporting

| Field | Value |
|-------|-------|
| Status | skeleton |
| Priority | P2 |
| Release reason | Thomas put this fleet capability (device health and inventory reports to the hub) in the first release; existing supported behavior and its obligations remain required. |
| Depends | plan/release/features/spec-fleet-1-device-registry.md |
| Phase | 4/10 of plan/post-release/features/spec-fleet-0-umbrella.md |
| Updated | 2026-11-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/managed/client.go` -- RunManagedClient (add reporting here)
4. `internal/core/health/` -- health registry (source data)
5. `internal/component/host/inventory.go` -- hardware inventory (source data)
6. `plan/post-release/features/spec-fleet-0-umbrella.md` -- umbrella design decisions
7. `plan/release/features/spec-fleet-1-device-registry.md` -- device registry (dependency)

## Task

Add two new RPC verbs (`inventory-report` and `health-report`) so managed devices push
their hardware inventory and component health to the hub. The hub stores this data in
the device registry and exposes it through commands on every command surface (CLI, SSH,
MCP, REST, gRPC, the web command line).

### Owner ruling of 2026-11-23: commands in the first release, pages after it

Thomas ruled (voice-transcribed answer "1 b") that the fleet features of the first
release are reached through commands on every command surface, and that their web
pages and views come after the first release (`ai/META-DECISIONS.md`, "Goal and
scope"). This spec keeps the verbs, the hub's storage and the three commands; every web
view it named moved to the post-release fleet-ui spec that owns the page, and the
inventory-health part of the fleet interface (which `spec-fleet-1-device-registry.md`
had given this spec) moved to `fleet-ui-2`, which builds it over this spec's commands.

| Moved out | Destination spec | Its AC there |
|-----------|------------------|--------------|
| AC-4, its web half ("fleet dashboard shows degraded") | `plan/post-release/features/spec-fleet-ui-3-dashboards-and-charts.md` | AC-8 |
| AC-12, the inventory and health columns of the device table and the side panel | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` | AC-16 |
| AC-12, the device detail page with full inventory and health | `plan/post-release/features/spec-fleet-ui-4-device-tabs-and-console.md` | AC-8 |
| The `InventoryHealthPart` registration (from `fleet-1`'s part table) | `plan/post-release/features/spec-fleet-ui-2-estate-tree-and-device-tables.md` | AC-17 |

AC-4 keeps its command half, and AC-13, added with this ruling, proves the three
commands on every command surface, so what the moved views showed stays reachable and
tested in the first release.

Currently each device has rich local data (CPU, NIC, DMI, memory, thermal, storage, SMART
via `host/inventory.go`; component health via `core/health/`) but none of it is reported
to the hub. An operator cannot answer "which devices have ECC errors?", "which devices
have degraded BGP?", or "what hardware is in edge-42?" without SSHing to each device.

This is the first spec that modifies the client side. The managed client loop gains two
periodic report calls. The protocol gains two new verbs on the existing MuxConn transport.

### Key Design Decisions (from umbrella)

| Decision | Detail |
|----------|--------|
| New RPC verbs | `inventory-report` and `health-report` on existing `#id verb [json]\n` framing |
| Client pushes, hub stores | Device initiates reports; hub is passive receiver. No hub-to-device query |
| Report timing | Inventory: on connect + daily. Health: on connect + every 5 minutes |
| Data scope | Inventory summary (no raw SMART data). Health: same structure as `/health` HTTP endpoint |
| Storage | Per-device fields in DeviceRegistry (from fleet-1) |

### Post-wave corrections (2026-07-10)

New obligation from the 2026-07 implementation wave (verified against current code): the new
report verbs ride a transport that now enforces write timeouts. `pkg/plugin/rpc/conn.go`
applies a default 30s write deadline when the context carries none (`defaultWriteDeadline`,
conn.go; applied in `writeAppended`, conn.go, :309). The managed client wraps its
TLS `net.Conn` in `rpc.NewConn` (`internal/component/managed/client.go`), a
deadline-capable transport, so `inventory-report` and `health-report` writes get the 30s
deadline (the fail-fast watchdog for non-deadline transports, conn.go, never arms
here). Risk is low: the ~1-2KB payloads noted under Key insights are far below the 16 MB
`MaxMessageSize` frame bound (`pkg/plugin/rpc/framing.go`). Obligation for the Data Flow
and client-side error handling: a report write that stalls 30s now fails with a deadline
error / closed connection instead of blocking; treat it like the AC-11 error case (client
continues operating normally and lets the existing reconnect logic recover).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/fleet-config.md` -- fleet protocol
  -> Constraint: new verbs follow same `#id verb [json]\n` framing
- [ ] `internal/core/health/health.go` -- health registry: Report() returns component checks
  -> Decision: health-report payload matches Report() output structure
- [ ] `internal/component/host/inventory.go` -- Detect() returns hardware inventory
  -> Decision: inventory-report sends summary subset, not full Detect() output

**Key insights:**
- Health Report() returns `map[string]Check` with Name, Status (healthy/degraded/down), Message
- Inventory Detect() returns CPUs, NICs, DMI, Memory, Thermal, Storage with detailed fields
- Inventory summary: CPU count + model, NIC count + drivers, total memory, storage count + total size
- Health report is small (~1KB); inventory summary is small (~2KB). No bandwidth concern

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/managed/client.go` -- RunManagedClient runs heartbeat + notification loop; only handles config-changed and ping verbs from hub
  -> Constraint: report calls added to client loop after successful config-fetch
- [ ] `internal/core/health/health.go` -- health.Registry with Register/Report/ServeHTTP
  -> Decision: client calls Report() and serializes to JSON for health-report RPC
- [ ] `internal/component/host/inventory.go` -- host.Detect() with cached TTL
  -> Decision: client calls Detect() on connect, caches result, sends summary
- [ ] `pkg/fleet/envelope.go` -- four existing verb constants
  -> Decision: add VerbInventoryReport and VerbHealthReport constants

**Behavior to preserve:**
- Existing four RPC verbs unchanged
- Client heartbeat and reconnect behavior unchanged
- Local health HTTP endpoint unchanged
- Local inventory detection unchanged

**Behavior to change:**
- Client loop: after config-fetch, send inventory-report; periodically send health-report
- Two new RPC verbs in `pkg/fleet/`
- Hub-side handlers store reports in DeviceRegistry
- Commands on every command surface: `show fleet device inventory <name>`, `show fleet device health <name>`, `show fleet health`
- Web: none in this spec; the inventory and health views are `fleet-ui-2`'s, `fleet-ui-3`'s and `fleet-ui-4`'s, after the first release (owner ruling of 2026-11-23)

## Data Flow (MANDATORY)

### Entry Point
- Client connects, completes config-fetch, then sends inventory-report
- Client sends health-report every 5 minutes
- Operator queries through the commands, on any command surface

### Transformation Path
1. Client calls `host.Detect()` to get local inventory (cached)
2. Client serializes summary to JSON, sends `inventory-report` RPC
3. Hub handler parses payload, stores in DeviceRegistry for that device
4. Client calls `health.Report()` to get component health
5. Client serializes to JSON, sends `health-report` RPC
6. Hub handler parses payload, stores in DeviceRegistry
7. The commands read the registry for inventory/health per device or aggregated, through the dispatcher, the same answer on every command surface

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Client local data to RPC | JSON serialization of inventory/health | [ ] |
| Client to Hub | `inventory-report` and `health-report` verbs on MuxConn | [ ] |
| Hub handler to DeviceRegistry | Store report data per device | [ ] |
| DeviceRegistry to every command surface | Builtin commands through the daemon dispatcher (CLI, SSH, MCP, REST, gRPC, the web command line) | [ ] |

### Integration Points
- `RunManagedClient` -- add report calls to client loop
- `pkg/fleet/envelope.go` -- new verb constants and payload types
- `ManagedConfigService` -- new verb dispatch (or separate fleet handler)
- `DeviceRegistry` from fleet-1 -- store inventory and health per device
- Commands -- `show fleet device inventory <name>`, `show fleet device health <name>`, `show fleet health`, registered once and reached on every command surface
- Web -- none here: the views moved to `fleet-ui-2`, `fleet-ui-3` and `fleet-ui-4` (owner ruling of 2026-11-23)

### Architectural Verification
- [ ] No bypassed layers (reports use existing MuxConn transport)
- [ ] No unintended coupling (hub ignores unknown verbs; old hubs safe with new clients)
- [ ] No duplicated functionality (reuses existing health.Report() and host.Detect())
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Client connects and completes config-fetch | -> | Client sends inventory-report | `test/managed/fleet-inventory-report.ci` |
| Client health timer fires | -> | Client sends health-report | `test/managed/fleet-health-report.ci` |
| `show fleet device inventory edge-01` | -> | Hub returns stored inventory | `test/managed/fleet-inventory-report.ci` |
| `show fleet health` | -> | Hub aggregates health across devices | `test/managed/fleet-health-report.ci` |
| The three commands on SSH, MCP, REST, gRPC and the web command line | -> | The same builtins through the daemon dispatcher | `TestEveryCommandOnEverySurface` (`cmd/ze/hub/surface_parity_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Client connects and completes config-fetch | Client sends `inventory-report` RPC with hardware summary |
| AC-2 | Hub receives inventory-report | Inventory stored in DeviceRegistry for that device |
| AC-3 | Client health timer fires (every 5 minutes) | Client sends `health-report` RPC with component statuses |
| AC-4 | Hub receives health-report with degraded component | Device health updated in registry; `show fleet device health <name>` shows the component degraded and `show fleet health` counts the device degraded (the dashboard half moved to `fleet-ui-3` AC-8) |
| AC-5 | `show fleet device inventory edge-01` | Shows CPU, NIC, memory, storage summary for edge-01 |
| AC-6 | `show fleet device health edge-01` | Shows per-component health status (healthy/degraded/down) |
| AC-7 | `show fleet health` | Aggregated view: N healthy, N degraded, N down. Lists degraded/down devices |
| AC-8 | Device has not reported yet (first connect in progress) | Inventory and health show "pending" or "not reported" |
| AC-9 | Hub receives inventory-report from a declared client (`fleet-1` admits only a client its hub declares, on the address of the block that declares it) | Report accepted and stored under the name the session authenticated as. A name no client entry declares is refused at authentication (`AuthenticateWithLookup` refuses an empty expected token), so no report from it is ever stored; `fleet-1` has no auto-discovery (its table "Superseded from the May skeleton") |
| AC-10 | Old hub (without fleet-4) receives inventory-report | Hub ignores unknown verb (existing behavior for unknown verbs) |
| AC-11 | Client connected to old hub (without fleet-4) | Client sends report, gets error or no response; continues operating normally |
| AC-13 | `show fleet device inventory edge-01`, `show fleet device health edge-01` and `show fleet health` asked on the CLI, SSH, MCP, REST, gRPC and the web command line | Each surface lists the three commands and the daemon dispatcher serves them there (`TestEveryCommandOnEverySurface`); every surface's answer is the builtin handler's, whose content `fleet-inventory-report.ci` and `fleet-health-report.ci` assert. Added by the owner ruling of 2026-11-23 |

AC-12 (the web dashboard's inventory and health columns and the device detail page)
moved: the table and side panel to `fleet-ui-2` AC-16, the detail page to `fleet-ui-4`
AC-8 (the Task section's ruling table).

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInventoryReportPayload` | `pkg/fleet/report_test.go` | Inventory report JSON marshal/unmarshal | |
| `TestHealthReportPayload` | `pkg/fleet/report_test.go` | Health report JSON marshal/unmarshal | |
| `TestInventoryReportHandler` | `internal/component/plugin/server/fleet_report_test.go` | Hub stores inventory from report | |
| `TestHealthReportHandler` | `internal/component/plugin/server/fleet_report_test.go` | Hub stores health from report | |
| `TestClientSendsInventoryReport` | `internal/component/managed/report_test.go` | Client sends inventory after config-fetch | |
| `TestClientSendsHealthReport` | `internal/component/managed/report_test.go` | Client sends health periodically | |
| `TestFleetHealthAggregation` | `internal/component/plugin/server/fleet_report_test.go` | Aggregate health across multiple devices | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `fleet-inventory-report` | `test/managed/fleet-inventory-report.ci` | Device connects, reports inventory, queryable via CLI | |
| `fleet-health-report` | `test/managed/fleet-health-report.ci` | Device reports health, aggregated view via CLI | |

## Files to Modify
- `pkg/fleet/envelope.go` -- add VerbInventoryReport, VerbHealthReport constants
- `internal/component/managed/client.go` -- add report calls to client loop
- `internal/component/plugin/server/managed.go` -- dispatch new verbs to handlers
- `internal/component/plugin/server/registry.go` -- add inventory/health fields to device record
- `cmd/ze/hub/main.go` -- wire report handlers

## Files to Create
- `pkg/fleet/report.go` -- InventoryReport and HealthReport payload types
- `pkg/fleet/report_test.go` -- payload marshal/unmarshal tests
- `internal/component/managed/report.go` -- client-side report sending logic
- `internal/component/managed/report_test.go` -- client-side tests
- `internal/component/plugin/server/fleet_report.go` -- hub-side report handlers
- `internal/component/plugin/server/fleet_report_test.go` -- hub-side tests
- `test/managed/fleet-inventory-report.ci` -- functional test
- `test/managed/fleet-health-report.ci` -- functional test

## Implementation Steps

### Implementation Phases

1. **Phase: Payload types** -- InventoryReport, HealthReport in pkg/fleet/
   - Tests: `TestInventoryReportPayload`, `TestHealthReportPayload`
   - Files: `report.go`, `report_test.go`
   - Verify: JSON round-trip tests pass

2. **Phase: Hub-side handlers** -- parse reports, store in DeviceRegistry
   - Tests: `TestInventoryReportHandler`, `TestHealthReportHandler`, `TestFleetHealthAggregation`
   - Files: `fleet_report.go`, `fleet_report_test.go`, `registry.go`, `managed.go`
   - Verify: reports stored and queryable

3. **Phase: Client-side reporting** -- send reports after config-fetch and periodically
   - Tests: `TestClientSendsInventoryReport`, `TestClientSendsHealthReport`
   - Files: `report.go` (managed), `report_test.go` (managed), `client.go`
   - Verify: client sends reports on connect and periodically

4. **Phase: Commands** -- the inventory and health commands, on every command surface (the web views are `fleet-ui-2`, `fleet-ui-3` and `fleet-ui-4`'s, owner ruling of 2026-11-23)
   - Tests: `fleet-inventory-report.ci`, `fleet-health-report.ci`, `TestEveryCommandOnEverySurface` (AC-13)
   - Files: the command registrations and their YANG `ze:command` nodes
   - Verify: `show fleet device inventory/health <name>` and `show fleet health` work on every command surface, run with `./le test managed`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N have implementation |
| Correctness | Report failure does not disrupt config-fetch or heartbeat |
| Naming | RPC verbs are `inventory-report` and `health-report` (kebab-case) |
| Data flow | Client local data -> RPC -> hub handler -> registry -> commands on every command surface |
| Rule: backward-compat | Old hub ignores new verbs; new client tolerates error responses |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Report payloads bounded (max 64KB); field counts validated |
| Rate limiting | Max one inventory report per connect; health report interval enforced (no faster than 1 minute) |
| Information exposure | Inventory summary only (no raw SMART data, no serial numbers beyond what DMI provides) |

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
- [ ] AC-1..AC-11 and AC-13 all demonstrated (AC-12 moved to `fleet-ui-2` AC-16 and `fleet-ui-4` AC-8)
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`)

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
