# Spec: vpp-telemetry-stat-groups

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P3 |
| Release reason | VPP telemetry ships with interface, node and system groups; the other stat-segment groups are added monitoring |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze already exports VPP statistics to its Prometheus endpoint: a poller reads
the VPP stats segment and publishes interface counters, graph-node counters,
the system vector and input rates, and `ze_vpp_stats_up`. An operator running
VPP also needs the error counters (why packets drop), buffer-pool usage,
per-worker load and heap memory, and wants to choose which groups Ze exports.

VyOS reference: T8382 `f712f1de0` (`~/Code/github.com/vyos/vyos-1x`) added a VPP
Prometheus exporter with selectable groups `interfaces`, `err`,
`buffer-pools`, `system`, `workers`, `nodes`, `memory`, `nat44`, `acl`, and a
`per-node-counters` startup option; the `nodes` group warns without it. Ze's
poller covers `interfaces`, `nodes` and `system`. VyOS runs a separate exporter
process in a VRF; Ze exports through its own metrics registry, and the VRF
side is `plan/post-release/features/spec-vrf-management-listeners.md`.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/vpp.md`, `docs/guide/monitoring.md`, `docs/plugin-development/metrics.md` - what is exported, the naming rules
- [ ] `ai/digests/vpp-dataplane.md` - the stats connection
  → Constraint: a metric or label rename breaks dashboards (`ai/doors/api-surfaces.md`); existing `ze_vpp_*` names stay

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/vpp/telemetry.go` - `statsProvider` reads interface, node and system stats; `newVPPMetrics` registers `ze_vpp_interface_*`, `ze_vpp_node_{clocks,vectors,calls}`, `ze_vpp_system_{vector,input}_rate`, `ze_vpp_stats_up`; `statsPoller.poll` returns at the first group that fails and sets `ze_vpp_stats_up` to 0, so one failing group hides every other group
- [ ] `internal/component/vpp/config.go` - `StatsSettings{SegmentSize, SocketPath, PollInterval}`, poll interval 1 to 3600 s
- [ ] `internal/component/vpp/startupconf.go` - the `statseg` section writes `size`, `page-size`, `socket-name`; no `per-node-counters`

**Behavior to preserve:**
- Every existing metric name and label
- `ze_vpp_stats_up` meaning: 1 when the stats socket answers

**Behavior to change:**
- More groups, operator-selected, and one group's failure no longer hides the others

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| VPP stats poller | `internal/component/vpp/telemetry.go` `statsPoller.poll`, `newVPPMetrics` | Three groups into the metrics registry | Extends: more groups, each its own step |
| Stats settings | `internal/component/vpp/config.go` `StatsSettings` | Segment size, socket, poll interval | Extends: a group selection leaf-list and `per-node-counters` |
| VPP startup.conf builder | `internal/component/vpp/startupconf.go` `statseg` section | Writes the segment options | Extends: writes `per-node-counters on` when the operator asks |
| Metrics registry | `internal/core/metrics` `Registry` | Prometheus registration | Extends: no second exporter, no second port |
| govpp stats API | `vendor/go.fd.io/govpp/core` `StatsConnection` | Typed readers for interface, node, system, error, buffer, memory stats | Extends: the poller calls the readers it does not call yet |
| VPP stub | `plan/release/validation/spec-finish-vpp-stub.md` AC-13 | Serves a stats segment for tests | Extends: its seeded segment gains the new groups for the functional test |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: `vpp stats { group <name>; per-node-counters; }` (shape at design)

### Transformation Path
1. Startup: `per-node-counters on` in `statseg` when set
2. Each poll: every selected group is read on its own; a failure marks that group down and the others still update
3. Metrics are written to the registry under `ze_vpp_<group>_*`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to VPP | stats segment socket, govpp `StatsConnection` | No |
| Ze to Prometheus | the existing metrics endpoint | No |

### Changed-Wire Census
**Search evidence:** to write at design. New metric names only.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [to fill at design] | | | | |

### Integration Points
- `statsPoller`, `newVPPMetrics`, the startup.conf builder

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | to fill at design |
| No unintended coupling | No | stays in `component/vpp` |
| No duplicated functionality | No | one poller, one registry |
| Zero-copy preserved where applicable | No | reuses the govpp stat structs across polls, as today |
| Registration over hardcoding, outbound | No | to fill at design: a group table each reader registers into |
| Registration over hardcoding, inbound | No | to fill at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Without `per-node-counters`, VPP leaves the node stats empty or absent, so `GetNodeStats` returns nothing or an error | VyOS T8382 commit text | If it errors, today's `poll` reports `ze_vpp_stats_up` 0 and skips the interface and system metrics: a defect in shipped code, owed a journal row and a fix | run against real VPP with node counters off | unvalidated |
| A-2 | govpp `StatsConnection` reads error, buffer and memory stats | vendored `go.fd.io/govpp/core` | Ze reads those stat paths itself | read `vendor/go.fd.io/govpp/core/stats.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Error counters per node and per worker explode label cardinality | scrape size | export per error name summed over workers, workers as an opt-in group |

## Blast Radius
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Missing or wrong VPP metrics; nothing in forwarding |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-finish-vpp-stub.md` |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `vpp stats group errors` with the stub segment | → | the error-group reader in `statsPoller.poll` | `vpp-telemetry-groups.ci` (planned, `test/vpp/`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Default config | The existing interface, node and system metrics, plus errors, buffer pools and memory, are exported; existing names unchanged | None |
| AC-2 | Operator selects groups | Only the selected groups are exported; selecting any group replaces the default set | None |
| AC-3 | Node group selected without `per-node-counters` | Commit warns with the leaf that enables it; interface and system metrics still update and `ze_vpp_stats_up` stays 1 | None |
| AC-4 | One group's read fails | That group's own up gauge is 0 and its metrics stop; every other group keeps updating | None |
| AC-5 | `per-node-counters` set | startup.conf `statseg` carries `per-node-counters on` | None |
| AC-6 | Error counter for a named VPP error | `ze_vpp_error_count{node,reason}` (names at design, per `metrics.md`) equals the stat segment value | None |
| AC-7 | VPP not configured, VPP telemetry selected | Refused at verify, naming the `vpp` block | None |
| AC-8 | Status of VPP telemetry from every surface | Shows each group and its state, the same everywhere | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPollGroupFailureIsolated` | `internal/component/vpp/telemetry_test.go` | AC-4 with a fake `statsProvider` | |
| `TestErrorGroupMetrics` | same | AC-6 | |
| `TestStartupConfPerNodeCounters` | `internal/component/vpp/startupconf_test.go` | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| poll-interval (existing) | 1-3600 | 1, 3600 | 0 | 3601 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-telemetry-groups` | `test/vpp/` | AC-1, AC-2 through the metrics endpoint on the stub | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | | | VPP is the data plane, not a peer; real-VPP proof runs in the VPP lab | |

## Files to Modify
- `internal/component/vpp/telemetry.go`, `internal/component/vpp/config.go`, `internal/component/vpp/startupconf.go`
- the VPP YANG module under `internal/component/vpp/yang/`
- `docs/guide/vpp.md`, `docs/guide/monitoring.md`

## Files to Create
- the tests above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | group leaf-list, `per-node-counters` |
| YANG validation constraints | Yes | enumeration of groups |
| YANG custom validators | Yes | node group without per-node counters warns |
| CLI commands/flags | Yes | telemetry status |
| CLI grammar (keyword before value) | N-A | no new verb |
| Editor autocomplete | Yes | enum |
| Functional test for new RPC/API | Yes | above |
| Pipe completeness | N-A | existing commands |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | Yes | stats socket unreachable while VPP runs |
| Prometheus counters/metrics | Yes | `ze_vpp_error_*`, `ze_vpp_buffer_*`, `ze_vpp_memory_*`, `ze_vpp_worker_*`, per-group up gauges |
| BGP family surface | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed | Yes | `docs/guide/vpp.md` |
| 14 | Prometheus counters | Yes | `docs/guide/monitoring.md`, `docs/plugin-development/metrics.md` |

## Implementation Steps
1. **Phase: Wiring** - group selection, isolation of failures (AC-4)
2. **Phase: New groups** - AC-1, AC-6
3. **Phase: Startup and status** - AC-3, AC-5, AC-7, AC-8

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Extend Ze's poller | A separate exporter process (VyOS) | One metrics endpoint, one auth and TLS setup |

## Known Limitations
- NAT44 and ACL groups depend on Ze driving those VPP plugins, which it does not today.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test vpp`, `./le job run label vpp command go test -tags ze_vpp ./internal/component/vpp/`
