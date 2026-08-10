# Spec: vrrp-deferred-keepalived-interop-scenarios

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:** this spec file;
`.claude/rules/planning.md`; `scripts/evidence/effective-vrrp-keepalived.py`
(the lab this spec extends, its module docstring is the topology);
`docs/architecture/vrrp/vrrp-first-hop-redundancy.md`.

## Task

Deferral holder. Provenance: `spec-vrrp-0-umbrella` (Goal Validation AC-3),
recorded 2026-07-16 in `plan/deferrals.md` row 72. The umbrella was closed and
removed, so this file is the work's home. The surviving
`plan/spec-vrrp-7-vpp.md` covers the VPP dataplane only; its interop row is
about the VPP path, not these kernel-path scenarios.

The keepalived interop lab `scripts/evidence/effective-vrrp-keepalived.py`
(driven by `make ze-qemu-vrrp-keepalived-test`, `mk/test-integration.mk`)
implements three scenarios and declares five more as not implemented. Verified
2026-07-16:

| Scenario | State | Evidence |
|----------|-------|----------|
| QS-1 v3 IPv4 election (ze prio 200 vs keepalived prio 100) | implemented | `SCENARIOS` dict, `effective-vrrp-keepalived.py` |
| QS-2 node-death failover and ze preempt return | implemented | `:1629` |
| QS-3 graceful stop, Priority-0 skew path | implemented | `:1630` |
| QS-4 reverse mastership and preempt false | NOT IMPLEMENTED | `PENDING_SCENARIOS`, `:1637` |
| QS-5 v2 opt-in wire format vs default keepalived | NOT IMPLEMENTED | `:1638` |
| QS-6 IPv6 v3: link-local plus global VIP, unsolicited NA | NOT IMPLEMENTED | `:1639` |
| QS-7 duplicate-VRID tie-break by greater primary IP | NOT IMPLEMENTED | `:1640` |
| QS-8 ze vs ze election | NOT IMPLEMENTED | `:1641` |

The deferral row names QS-5 and QS-6, which is this spec's primary scope. The
row is incomplete rather than wrong: QS-4, QS-7 and QS-8 sit in the same
`PENDING_SCENARIOS` dict and have no other home, so they are tracked here too
and the design phase decides the order.

Supporting facts, verified:
- v3 IPv4 is proven against keepalived 2.3.1 (`effective-vrrp-keepalived.py`
  records the probe date 2026-07-15; `Lab.keepalived_version` at `:1117` reports
  the live version on failure). The implemented scenarios pin `vrrp_version 3`
  in the peer config because keepalived speaks v2 by default.
- v2 has codec coverage but no wire exchange with a peer: `TestEncodeGoldenV2`
  (`internal/plugins/vrrp/packet/packet_test.go`) asserts the exact golden
  bytes and `TestRoundTrip` covers v2 encode-decode, while the
  config-rejection path is covered by `test/vrrp/vrrp-config-invalid.ci`
  and `test/vrrp/vrrp-doctor-fires.ci`. No scenario puts a ze v2 advertisement in
  front of keepalived.
- IPv6 v3 is not in the committed lab at all. The row says IPv6 interop was once
  demonstrated with ad-hoc scripts; nothing in the repo automates it, which is
  precisely the gap.

This is test infrastructure, not a protocol gap.

## Required Reading

### Architecture Docs
- [ ] `scripts/evidence/effective-l2tp-ppp.py` - the blueprint the vrrp lab follows (PID-suffixed names, netns setup and cleanup, LineCollector predicate waits, diagnostics on failure)
  → Constraint: new scenarios follow the same shape, no new lab harness
- [ ] `ai/rules/interop-and-goal-validation.md` - when interop evidence is required and what counts
  → Constraint: proof comes from outside ze (tcpdump wire fields, keepalived notify markers, ping exit codes); ze log lines are readiness markers only
- [ ] `ai/rules/platform-linux.md` - QEMU integration tests are mandatory for linux-only code, never skipped for "needs hardware"
  → Constraint: these run in the stock Alpine VM via `make ze-qemu-vrrp-keepalived-test`

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3768.md` - the v2 wire format QS-5 exercises
  → Constraint: v2 carries the interval in whole seconds and the Auth Type/Data fields v3 dropped; the golden bytes at `packet_test.go` are the encode contract
- [ ] `rfc/short/rfc9568.md` - v3 for both families, and the v2/v3 relationship
  → Constraint: no on-the-wire compatibility between v2 and v3 (§7.1 discards the other version), so QS-5 must pin keepalived to v2, not mix versions (`rfc/short/rfc9568.md`)

**Key insights:**
- The lab's third-party observer namespace already sees every flooded frame, so IPv6 multicast (ff02::12) needs no new topology (`effective-vrrp-keepalived.py` notes the existing setup covers it).
- `print_scenarios` prints implemented and pending scenarios together, so each landing scenario moves one row from `PENDING_SCENARIOS` into `SCENARIOS`.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-16)
- [ ] `scripts/evidence/effective-vrrp-keepalived.py` - three-leaf netns lab plus bridge; `SCENARIOS` holds QS-1..QS-3 (:1625-1631), `PENDING_SCENARIOS` holds QS-4..QS-8 as NOT IMPLEMENTED (:1636-1642); IPv4-only constants (VIP 192.0.2.1, VRID 10, v3, :91-118); keepalived peer pinned to `vrrp_version 3` (:786-812)
- [ ] `internal/plugins/vrrp/packet/packet_test.go` - `TestEncodeGoldenV2` (:86) asserts the v2 golden bytes (:23); `TestRoundTrip` (:170) covers v2, v3 IPv4 and v3 IPv6
- [ ] `internal/plugins/vrrp/packet/packet.go` - one codec for both versions; `msToV2Seconds` (:237) and `msToV3Centiseconds` (:230) are the per-version interval encodings QS-5 puts on the wire
- [ ] `mk/test-integration.mk` - `ze-qemu-vrrp-keepalived-test` runs the script in the stock Alpine VM (:443-447)
- [ ] `test/vrrp/vrrp-config-invalid.ci` - the v2 cross-leaf config rejections (:185-208), config-plane only

**Behavior to preserve:**
- QS-1, QS-2 and QS-3 keep passing unchanged; new scenarios are additive
- The lab's evidence discipline: assertions come from tcpdump wire fields, keepalived notify markers, `ip -j neigh` and ping exit codes, never from ze's own state output
- PID-suffixed namespace and device names (device names must fit IFNAMSIZ-1, `:55-59`), setup and cleanup ownership in `main()`, artifacts kept on failure
- `print_scenarios` listing implemented and pending together, so the remaining gap stays visible
- The existing v2 codec and config-rejection unit and functional coverage

**Behavior to change:**
- QS-5 and QS-6 move from `PENDING_SCENARIOS` into `SCENARIOS` with real scenario functions
- The lab's IPv4-only constants gain an IPv6 counterpart and the keepalived config generator gains a v2 form

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `make ze-qemu-vrrp-keepalived-test` (`mk/test-integration.mk`) boots the stock Alpine VM and runs `python3 scripts/evidence/effective-vrrp-keepalived.py`
- The script also takes scenario names on the command line and `--list` (`usage`, `:1652`), so a single scenario can be run alone

### Transformation Path
1. `main()` builds the four namespaces, veths and bridge, then probes kernel support (module docstring `:13-23`, `ensure_kernel_support`)
2. A generated ze config and a generated keepalived config start both routers on the shared segment
3. tcpdump on the observer namespace captures every advertisement; keepalived's notify script writes the state marker file (`MARKER_NAME`, `:81`)
4. Each scenario function asserts wire fields, timing bands measured from tcpdump timestamps, and reachability, then raises `RuntimeError` on failure
5. `main()` owns teardown and the PASS marker; artifacts are kept when a scenario fails

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| make ↔ QEMU VM | `ze-qemu-vrrp-keepalived-test` runs the script as root in Alpine | [ ] |
| ze ↔ keepalived | VRRP advertisements on the shared L2 segment (multicast 224.0.0.18 for v4; ff02::12 for v6, new in QS-6) | [ ] |
| lab ↔ ze | generated config file plus state-change log lines used as readiness markers only | [ ] |
| lab ↔ keepalived | generated config plus the notify marker file | [ ] |
| lab ↔ wire truth | tcpdump fields and timestamps; `ip -j neigh`; ping exit codes | [ ] |

### Integration Points
- `SCENARIOS` / `PENDING_SCENARIOS` dicts: each landing scenario is a `scenario_qsN(lab)` moved from one dict to the other
- The `Lab` class helpers (`keepalived_version` at `:1117`, `ka_state`, the LineCollector predicate waits) are reused as-is
- The keepalived config generator gains a v2 variant for QS-5 and an IPv6 variant for QS-6
- `internal/plugins/vrrp/packet/packet.go` interval encodings: what QS-5 proves on the wire against a real v2 peer

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | keepalived 2.3.1 speaks v2 well enough to pair with ze's v2 opt-in | keepalived defaults to v2, which the lab currently pins away from | QS-5 shrinks to a capture-only proof or a different peer | Design phase: run keepalived without `vrrp_version 3` and capture | unvalidated |
| A-2 | The existing bridge topology carries IPv6 VRRP multicast with no change | `:348` states the setup covers the ff02::12 case | QS-6 needs topology work | Design phase: capture ff02::12 on the observer | unvalidated |
| A-3 | The stock Alpine kernel needs no extra modules for the IPv6 path | `ensure_kernel_support` probes macvlan, bridge and veth only | The VM needs a custom kernel, which the target explicitly avoids | Extend the probe and run | unvalidated |
| A-4 | ze's IPv6 first-advert source quirk does not red QS-6 | `docs/architecture/vrrp/vrrp-first-hop-redundancy.md`: the first IPv6 advert sources from the transient EUI-64 link-local, judged cosmetic and not fixable by action ordering | QS-6 asserts a source address that will not hold; assert on later adverts | Read learned 1122 before writing the assertion | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scenarios become flaky under timing bands measured in a VM | Intermittent reds in CI | Keep the existing predicate-wait discipline, band from wire timestamps, never sleep-and-hope |
| R-2 | QS-5 proves only that two v2 speakers agree, not that ze's v2 is correct | Scenario passes with both sides misreading the interval | Assert the raw wire fields against the golden encoding, not just the election outcome |
| R-3 | Scope creep into QS-4/QS-7/QS-8 stalls the two scenarios the deferral names | Design phase expands the table | Land QS-5 and QS-6 first; the other three stay listed as pending |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-qemu-vrrp-keepalived-test` | → | `SCENARIOS` dict dispatch, `scenario_qs5` | `QS-5` scenario in `scripts/evidence/effective-vrrp-keepalived.py` |
| `make ze-qemu-vrrp-keepalived-test` | → | `SCENARIOS` dict dispatch, `scenario_qs6` | `QS-6` scenario in `scripts/evidence/effective-vrrp-keepalived.py` |
| `effective-vrrp-keepalived.py --list` | → | `print_scenarios` over both dicts | QS-5 and QS-6 listed as implemented, not pending |

## Acceptance Criteria

Skeleton level; the design phase expands these.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | QS-5: ze with `version 2`, keepalived at its v2 default, same VRID and segment | Election completes; ze's v2 advertisements match the golden wire shape (interval in whole seconds, Auth Type 0) as captured by tcpdump |
| AC-2 | QS-6: ze and keepalived both v3 IPv6, link-local plus global VIP | Election completes; unsolicited NA observed after failover; the neighbor entry resolves to the virtual MAC |
| AC-3 | `--list` after both land | QS-5 and QS-6 appear in `SCENARIOS`, absent from `PENDING_SCENARIOS` |
| AC-4 | Full lab run | QS-1..QS-3 still pass unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (existing) `TestEncodeGoldenV2` | `internal/plugins/vrrp/packet/packet_test.go` | the v2 byte shape QS-5 asserts on the wire; already passing, referenced as the contract | |
| (existing) `TestRoundTrip` | `internal/plugins/vrrp/packet/packet_test.go` | v2 and v3 IPv6 codec coverage the scenarios rely on | |
| new codec test only if QS-5 or QS-6 exposes a real wire defect | `internal/plugins/vrrp/packet/` | regression for whatever the interop run finds | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| QS-5 v2 advert interval field (whole seconds, RFC 3768) | 1-255 | 255 | 0 | 256 |
| QS-6 v3 advert interval field (centiseconds, RFC 9568) | 1-4095 | 4095 | 0 | 4096 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (existing) `vrrp-config-invalid.ci` | `test/vrrp/` | v2 cross-leaf config rejections; unchanged by this spec, re-run as a regression | |

Note: the deliverable here is the interop lab, not new `.ci` files. The `.ci`
suite covers the config and show planes; wire exchange with a foreign daemon is
the QEMU lab's job (`ai/rules/interop-and-goal-validation.md`).

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| QS-5 v2 opt-in wire format | `scripts/evidence/effective-vrrp-keepalived.py` | keepalived (v2 default) | ze's RFC 3768 encoding interoperates with a real v2 peer | |
| QS-6 IPv6 v3 link-local plus global VIP | same | keepalived (v3, IPv6) | ze's IPv6 advertisement, election and unsolicited NA interoperate | |

### Future (if deferring any tests)
- QS-4, QS-7 and QS-8 stay in `PENDING_SCENARIOS` unless the design phase pulls them in. They remain tracked by this spec; do not let them fall out of the dict.

## Files to Modify
- `scripts/evidence/effective-vrrp-keepalived.py` - QS-5 and QS-6 scenario functions, v2 and IPv6 keepalived config generators, IPv6 constants, dict moves
- `mk/test-integration.mk` - only if the run needs a longer timeout or an extra kernel probe
- `docs/functional-tests.md` - the lab's scenario inventory
- `docs/features/rfc-status.md` - RFC 3768 and RFC 9568 IPv6 rows gain interop evidence

## Implementation Steps

Stage mapping follows `plan/TEMPLATE.md` unchanged.

### Implementation Phases

1. **Phase: Wiring (MANDATORY FIRST)** -- register QS-5 in `SCENARIOS` as a failing stub, prove `--list` and dispatch reach it
2. **Phase: QS-5** -- v2 keepalived config generator, v2 ze config, wire-field assertions against the golden shape
3. **Phase: QS-6** -- IPv6 constants and addressing, v3 IPv6 configs both sides, election plus unsolicited NA plus neighbor-entry assertions
4. **Phase: Regression** -- full lab run, QS-1..QS-3 unchanged
5. **Full verification** -- `make ze-qemu-vrrp-keepalived-test`, then `make ze-verify`
6. **Complete spec** -- audit, learned summary, two-commit closure

### Failure Routing
| Failure | Route To |
|---------|----------|
| keepalived rejects the generated config | Read the version it reports (`keepalived_version`, `:1117`); keyword drift, not a ze defect |
| ze v2 advert differs from the golden bytes | A real codec defect: add the unit regression first, then fix |
| IPv6 first-advert source assertion fails | A-4: read `docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md` before re-deriving; assert on later adverts |
| 3 fix attempts fail | STOP. Report all 3. Ask user. |

## Known Limitations
- Skeleton: no design done. Scenario shapes and assertion sets are open.
- Test infrastructure only. If a scenario finds a protocol defect, that fix is separate work with its own regression test.
- QS-4, QS-7 and QS-8 are in scope for tracking but not for the first pass.

## RFC Documentation

At implementation: each scenario carries the RFC citation for the field it
asserts, matching the lab's existing style (for example
`effective-vrrp-keepalived.py` cites RFC 9568 Sections 5.2.7, 7.3 and
5.1.1.3 above the constants they pin). Update the RFC 3768 and RFC 9568 IPv6
rows in `docs/features/rfc-status.md` with the new interop evidence.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 all demonstrated
- [ ] Wiring Test table complete, every row a concrete scenario name
- [ ] `/ze-review` gate clean (0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes
- [ ] `make ze-qemu-vrrp-keepalived-test` passes with QS-1..QS-3, QS-5 and QS-6
- [ ] Feature code integrated (`scripts/evidence/effective-vrrp-keepalived.py`, `mk/test-integration.mk`)
- [ ] Documentation Update Checklist answered with source evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled

### Completion (BLOCKING -- before ANY commit)
- [ ] Implementation Summary and Audit filled
- [ ] Learned summary written
- [ ] Two-commit closure per `ai/rules/planning.md`
