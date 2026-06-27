# Spec: Egress CS6 Priority Scheduling (Gap C)

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-cp-survival-0-umbrella |
| Phase | - |
| Updated | 2026-06-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugins/traffic/netlink/translate_linux.go:115-141` (the `translateFilter` bug)
4. `internal/component/traffic/model.go` (QdiscPrio, FilterDSCP, TrafficClass.Priority)
5. `test/vlan-qos-lab/` (existing DSCP-filter lab — currently non-functional)

## Task

Make Ze's **own egress** honor the DSCP CS6 mark it already puts on BGP packets, so that when Ze is
the congested node, BGP keepalives and the FlowSpec/RTBH UPDATE win the link instead of competing
equally with attack traffic.

Investigation found a **latent bug**: `translateFilter` builds a `netlink.U32` filter for
`FilterDSCP`/`FilterProtocol` with `ClassId` set but **no `Sel *TcU32Sel` match selector**
(`translate_linux.go:133-138`). A DSCP filter configured today reaches the kernel matching nothing —
it silently fails to classify. This spec (1) fixes the u32 selector so DSCP classification actually
works, and (2) ships a reference egress config that places CS6-marked control traffic in a
strict-priority class.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - the filter is half-implemented; fix at source
  → Constraint: populate the real `TcU32Sel`; do not paper over with a mark-based detour.
- [ ] `ai/rules/qemu-testing.md` - tc behavior is linux-only
  → Constraint: classification proof needs a netns/QEMU integration test (class counters), not just object construction.
- [ ] `docs/architecture/cli/color-system.md` - N/A (no TUI)
  → Constraint: skip; listed only to confirm not relevant.

**Key insights:**
- The fix is in `translate_linux.go`: set `U32.Sel = &netlink.TcU32Sel{ Keys: [...] }` matching the
  IPv4 TOS byte (offset 1) or the IPv6 traffic-class nibbles. DSCP value `d` → match `(d<<2)` with
  mask `0xFC` on the TOS byte (top 6 bits; bottom 2 are ECN).
- DSCP is stored as raw `uint32` in the traffic model; CS6 = 48 (TOS byte 0xC0, matching the BGP
  socket's `IP_TOS=0xC0`). There is no named-DSCP map today.
- VPP backend rejects all filters by design (`verify.go`); CS6 priority is a **tc/netlink-backend
  feature**. This is a documented constraint, not a regression to fix here.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/traffic/model.go:37-52,101-109,146-172` - `QdiscPrio` (49) exists; `FilterDSCP`/`FilterProtocol`/`FilterMark` (101-109); `TrafficClass{Rate,Ceil,Priority,Filters}`; `TrafficFilter{Type,Value uint32}`.
  → Constraint: DSCP value is raw uint32; class has a `Priority` field already.
- [ ] `internal/plugins/traffic/netlink/translate_linux.go` (lines 115-141) - **the bug**: `FilterDSCP`/`FilterProtocol` → `&netlink.U32{FilterAttrs, ClassId}` with no `Sel`. `FilterMark` → `FwFilter` (works).
  → Constraint: this is the single change that makes DSCP classification real.
- [ ] `internal/plugins/traffic/netlink/translate_linux.go:55-65` - `QdiscPrio` → `&netlink.Prio{Bands:3}` with no band-to-class mapping.
  → Constraint: PRIO path is incomplete; prefer HTB class-priority for the reference config.
- [ ] `internal/component/traffic/config.go:167-179` - `parseFilterValue` parses FilterDSCP/Protocol as decimal uint32.
  → Constraint: extend (optionally) to accept named `cs6`/`ef`; keep decimal back-compat.
- [ ] `internal/plugins/traffic/vpp/verify.go:15-17,176-183` - VPP rejects dscp/protocol/mark filters and multi-class HTB/TBF.
  → Constraint: document that CS6 priority requires the tc backend; VPP unchanged.
- [ ] `internal/component/traffic/yang/ze-traffic-control-conf.yang:14-43,111-124` - qdisc-type enum incl `prio`; filter-type enum `mark/dscp/protocol`; class `match{type,value}`.
  → Constraint: schema already supports a DSCP filter; this spec makes it functional.
- [ ] `test/vlan-qos-lab/ze-vlan-qos.conf` - shows a DSCP filter in traffic-control (proof-of-concept that currently does not classify due to the bug).
  → Constraint: this lab becomes a real regression witness after the fix.

**Behavior to preserve:**
- `FilterMark` (FwFilter) classification unchanged.
- VPP backend rejection of filters unchanged.
- Existing qdisc/class apply flow (`backend.Apply`) unchanged in shape.

**Behavior to change:** `FilterDSCP`/`FilterProtocol` now install a real u32 match selector.

## Data Flow (MANDATORY)

### Entry Point
- Config `traffic-control { interface X { qdisc htb { class control { priority 0; match dscp 48 } default ... } } }`.

### Transformation Path
1. `config.go` parses class + `match dscp 48` → `TrafficFilter{Type:FilterDSCP, Value:48}`.
2. `backend.Apply` → `translateClass` (HTB class with Priority) + `translateFilter`.
3. **Fixed** `translateFilter`: `&netlink.U32{Sel: &TcU32Sel{Keys: [{Off:0, Val:(48<<2), Mask:0xFC00... }]}, ClassId}` (IPv4 TOS byte at IP-header offset 1; IPv6 traffic-class).
4. `netlink.FilterAdd` installs a u32 that actually matches CS6 packets → steered into the priority class.
5. Kernel: CS6-marked egress (BGP) dequeued ahead of best-effort under congestion.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config ↔ traffic model | `parseFilterValue` | [ ] |
| traffic model ↔ netlink | `translateFilter` (fixed Sel) | [ ] |
| netlink ↔ kernel tc | `FilterAdd` with TcU32Sel | [ ] |

### Integration Points
- `internal/plugins/traffic/netlink/translate_linux.go` `translateFilter` — the fix
- `internal/component/traffic/config.go` `parseFilterValue` — optional named-DSCP
- `internal/plugins/traffic/vpp/verify.go` — unchanged; documents the backend limit

### Architectural Verification
- [ ] No bypassed layers (config → model → backend, unchanged shape)
- [ ] No unintended coupling
- [ ] No duplicated functionality (fixes existing filter; no parallel path)
- [ ] Zero-copy preserved (N/A — tc config)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The vendored `vishvananda/netlink` `TcU32Sel`/`TcU32Key` correctly encode a TOS-byte match | netlink supports u32 selectors | filter still won't match | netns test: CS6 packet increments class counter | unvalidated |
| A-2 | TOS-byte offset for IPv4 is header offset 1; IPv6 traffic-class spans bytes 0-1 | IP header layout | wrong bytes matched | integration test for both families | unvalidated |
| A-3 | HTB class `Priority` gives strict-ish dequeue preference adequate for control traffic | HTB prio semantics | CS6 not actually prioritized | netns congestion test: CS6 latency < best-effort under load | unvalidated |
| A-4 | The existing DSCP filter being non-functional means no production config silently depends on the broken behavior | bug analysis | fixing it changes classification for someone | grep configs/tests using dscp filter; vlan-qos-lab is the only one | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Fixing the selector changes behavior for an existing (broken) dscp filter user | a lab/test that "passed" now classifies differently | A-4 grep; vlan-qos-lab updated to assert real classification |
| R-2 | IPv6 traffic-class matching encoded wrong (it is not at the same offset as IPv4 TOS) | v6 packets unclassified | separate v4/v6 selector construction + per-family test |
| R-3 | Operators expect CS6 priority on VPP datapath but it is tc-only | VPP config rejected | clear error already exists; document in the guide |
| R-4 | u32 selector competes with existing FwFilter priorities | wrong filter wins | set explicit filter Priority; test ordering |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `match dscp 48` in an HTB class | → | `translateFilter` builds `U32{Sel:...}` | `TestTranslateFilterDSCPHasSelector` (unit) |
| CS6-marked packet on egress | → | u32 steers to priority class | `cs6-priority.ci` / netns class-counter test |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | HTB class with `match dscp 48` | `translateFilter` returns a `netlink.U32` whose `Sel` matches TOS byte `0xC0/0xFC` (top 6 bits) |
| AC-2 | netns: send 100 CS6-marked packets out the egress iface | The CS6 class byte/packet counter increments (classification works — the bug is fixed) |
| AC-3 | Reference config: HTB control class (high priority, CS6 filter) + default class, congested link | CS6 (BGP) packets observe lower latency/drop than best-effort under load |
| AC-4 | IPv6 egress with `match dscp 48` | IPv6 traffic-class CS6 packets are classified into the control class |
| AC-5 | Same DSCP filter under `backend vpp` | Rejected with the existing `errFilterDscpNotSupportedByBackend` message (constraint preserved) |
| AC-6 | Config `match dscp cs6` (named) | Parsed to value 48 (optional ergonomic; decimal `48` still accepted) |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a CS6 priority class on the uplink; under saturation BGP keeps flowing | config → model → netlink u32 (fixed) → kernel | `cs6-priority.ci` + netns congestion test |
| 2 | discovers their old dscp-filter config now actually classifies | the bug fix | `test/vlan-qos-lab` updated to assert class counters |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTranslateFilterDSCPHasSelector` | `internal/plugins/traffic/netlink/translate_linux_test.go` | U32.Sel populated with TOS-byte key (v4) | |
| `TestTranslateFilterDSCPv6Selector` | same | IPv6 traffic-class key | |
| `TestParseNamedDSCP` | `internal/component/traffic/config_test.go` | `cs6`→48, `ef`→46, decimal back-compat | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| dscp value | 0-63 | 63 | N/A (0 = BE, valid) | 64 |
| class priority | 0-7 | 7 | N/A | 8 (clamp/reject) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `cs6-priority` | `test/traffic/cs6-priority.ci` | configure CS6 control class; `tc filter show` includes the TOS match | |
| `dscp-classify` | `test/vlan-qos-lab/` (updated) | DSCP filter now classifies (regression witness) | |
| `TestCS6ClassifyNetns` | `internal/plugins/traffic/netlink/cs6_integration_linux_test.go` | netns: CS6 packets hit the class counter (QEMU) | |

### Interop Tests
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | local egress scheduling, not a wire protocol | |

## Files to Modify
- `internal/plugins/traffic/netlink/translate_linux.go` - **fix**: populate `U32.Sel` (TcU32Sel) for FilterDSCP/FilterProtocol, per-family
- `internal/component/traffic/config.go` - (optional) accept named DSCP `cs6`/`ef` in `parseFilterValue`
- `internal/plugins/traffic/netlink/translate_linux_test.go` - selector unit tests
- `test/vlan-qos-lab/ze-vlan-qos.conf` + assertions - turn the POC into a real regression test
- `docs/guide/` traffic/QoS guide - CS6 control-class recipe + VPP-backend caveat

## Files to Create
- `internal/core/dscp/dscp.go` - small named-DSCP map (cs6=48, ef=46, ...) if AC-6 taken; else skip
- `test/traffic/cs6-priority.ci` - functional test
- `internal/plugins/traffic/netlink/cs6_integration_linux_test.go` - netns classification test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | translate_linux.go current state |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Phases below |

### Implementation Phases
1. **Phase: Wiring (FIRST)** — write `TestTranslateFilterDSCPHasSelector` asserting `U32.Sel != nil` with the right key; it FAILS against current code (the bug).
2. **Phase: the fix (IPv4)** — populate `TcU32Sel` for the TOS byte; AC-1, AC-2.
3. **Phase: IPv6** — traffic-class selector; AC-4.
4. **Phase: reference config + priority** — HTB control class recipe; AC-3; netns congestion test.
5. **Phase: named DSCP (optional)** — `cs6`/`ef`; AC-6.
6. **Phase: VPP constraint** — assert rejection unchanged; AC-5; doc caveat.
7. **Full verification** → `make ze-verify-changed`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-N each has file:line |
| Correctness | TOS-byte offset/mask correct for v4 AND v6; CS6=48 → 0xC0 |
| Bug fix | a test proves the old behavior was broken and is now fixed |
| Rule: no-workarounds | real selector, not a mark detour |
| VPP | rejection message preserved |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | dscp 0-63; reject 64+; named map closed-set |
| Resource exhaustion | N/A (static tc config) |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Implementation Audit

| AC | Status | Evidence |
|----|--------|----------|
| AC-1 | Done | `translateFilter` returns `U32` with `TcU32Sel{Keys: [{Val: dscp<<18, Mask: 0x00FC0000}]}` for IPv4 |
| AC-2 | Done | `cs6_integration_linux_test.go` netns test sends CS6 packets, verifies class counter |
| AC-3 | Done | `021-cs6-priority-config.ci` functional test with HTB control + default class |
| AC-4 | Done | `dscpFilters` emits IPv6 key `{Val: dscp<<22, Mask: 0x0FC00000}`; dual-family test |
| AC-5 | Done | `020-vpp-reject-dscp-filter.ci` asserts VPP rejection message |
| AC-6 | Done | `dscp.Parse("cs6")` returns 48; `config_test.go` covers named + decimal |

## Review Gate
### Final status
- [x] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded
Post-commit closure: implementation committed as 5f2857128; spec closure deferred to this session.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/plugins/traffic`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)

### Completion (BLOCKING — before ANY commit)
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-cp-survival-3-egress-cs6-sched.md`
