# Spec: OSPFv3 NSSA Type-7 Redistribution and End-to-End v6 Redistribution Interop

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-ospf-af-unify |
| Phase | implementation complete; final verify pending |
| Updated | 2026-06-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc3101.md` (NSSA), `rfc/short/rfc2328.md` (sec 12.4.1 E-bit), `rfc/short/rfc5340.md` (OSPFv3)
4. `internal/plugins/ospf/origination_v6_external.go`, `internal/plugins/ospf/nssa.go`, `internal/plugins/ospf/lsdb/nssa.go`, `internal/plugins/ospf/redistribute/consumer.go`

## Task

Two deferred items from the completed `plan/spec-ospf-af-unify.md`:

**Part A -- OSPFv3 NSSA Type-7 redistribution.** When an OSPFv3 ASBR is attached to an NSSA area,
redistributed external IPv6 routes MUST be originated as **Type-7 NSSA-LSAs (0x2007) into the
NSSA** (RFC 3101), with the correct P-bit and forwarding address, and translated to **Type-5
AS-External-LSAs (0x4005)** at the NSSA ABR for the other areas. Today the v6 redistribution path
always originates Type-5 into the AS-wide store (`v6InjectExternal` -> `v6OriginateExternalLSA`),
which an NSSA blocks -- so an OSPFv3 ASBR sitting in an NSSA cannot inject externals into it.

**Part B -- full end-to-end v6 redistribution interop.** The scenario
`test/interop/scenarios/ospf-v6-redist-frr/` is currently PENDING (it forms an adjacency and
skip-passes). Driving redistribution end-to-end is blocked OUTSIDE the OSPFv3 engine by the
generic redistribution framework, and this spec must resolve that so FRR actually installs a
redistributed route.

**Prerequisite already resolved this session (NOT part of the remaining work).** The AF-agnostic
"am I an ASBR" / Router-LSA E-bit determination -- raised as ISSUE 1 (perf) and ISSUE 2 (v4/v6
divergence) in the `spec-ospf-af-unify` `/ze-review` -- is **fixed**:
`internal/plugins/ospf/lsdb/origination.go` now has `selfIsASBRLocked` / `SelfIsASBR` (a self-index
`d.own` walk that counts both Type-5 and Type-7, O(self-LSAs), AF-agnostic via
`.ASExternal()` / `.NSSA()`), and `internal/plugins/ospf/origination_v6.go:124` calls `SelfIsASBR`
(was `SelfExternalCount`). Regression: `TestOSPFASBRBitFromNSSAType7`. So once Part A originates a
v6 Type-7, the v6 NSSA ASBR will already set its Router-LSA E-bit correctly.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-ospf-af-unify.md` - the unified AF-aware engine this extends; the v6 seams
      (Transport/Codec/AFPrefixStrategy/Encoder) and the v6 Type-5 redistribution already built.
  → Constraint: v6 work must remain OSPFv2-byte-identical; v4 NSSA Type-7 origination + translation
    are already shipped and MUST stay behavior-identical (ze-ospf-test 13/13, ospf-stub-nssa-frr).
- [ ] `docs/research/ospf-implementation-guide.md` - NSSA model, translator election.
  → Decision: NSSA Type-7 is area-scoped; translation to Type-5 happens only at the elected ABR.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc3101.md` - The OSPF NSSA Option.
  → Constraint: §2.3/§2.4 an NSSA ASBR originates Type-7; the P-bit MUST be clear unless the LSA is
    translatable (non-zero forwarding address, not a default); the FA SHOULD be the ASBR's intra-NSSA
    interface address.
  → Constraint: §3.6 the elected ABR translates P-bit Type-7 to Type-5; a Type-7 with P=0 or FA=0 is
    NOT translated; do not translate a locally-originated Type-5's twin.
- [ ] `rfc/short/rfc2328.md` - OSPFv2 base.
  → Constraint: §12.4.1 the Router-LSA E-bit means "AS boundary router"; a Type-7 originator is an
    ASBR (already enforced AF-agnostically by `SelfIsASBR`).
- [ ] `rfc/short/rfc5340.md` - OSPFv3.
  → Constraint: OSPFv3 NSSA-LSA = function code 0x2007 (area scope); same semantics as v4 Type-7 but
    the v6 prefix/wire encoding (no embedded IPv4 mask; prefixes carry PrefixLength/Options).

**Key insights:**
- The v4 NSSA machinery (origination, P-bit policy, translation, FA) exists and works; Part A is
  largely "do the same through the v6 codec," not new protocol design.
- Part B's blocker is a non-OSPF framework concern; choose the smallest faithful interop.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ospf/origination_v6_external.go` - v6 redistribution origination. `v6InjectExternal`
      assigns a per-prefix Link State ID (`engine.redistV6`), validates the prefix, then
      `v6OriginateExternalLSA` originates a **Type-5 (0x4005)** into the AS-wide store via `OriginateSelf`
      (area = Backbone). No NSSA branch today.
  → Constraint: the LSID-per-prefix map (`redistV6`) and `v6ExternalSelfTypes` stale-flush set are the
    pattern to mirror for a Type-7 self set.
- [ ] `internal/plugins/ospf/nssa.go` - v4 NSSA control: `applyNSSADefaults` (now also on the 1s ticker),
      `externalScope` (returns NSSA attachments with per-area FA = intra-NSSA interface address),
      `translateNSSA` (Type-7 -> Type-5 with P=0 and FA=0 skip guards).
  → Constraint: `externalScope`'s `faByArea` already computes the correct FA per NSSA area; the v6 Type-7
    path should reuse it.
- [ ] `internal/plugins/ospf/lsdb/nssa.go` - `OriginateNSSA` / `PurgeNSSA` build a **v4-wire** `packet.LSA`
      (`External: &body`), enforce the P-bit at the origination boundary (P requires non-zero FA and no
      self Type-5 twin). This is v4-specific; a v6 equivalent (or an AF-aware encoder seam) is needed.
  → Constraint: keep the P-bit enforcement at the origination boundary for v6 too (no caller bypass).
- [ ] `internal/plugins/ospf/redistribute/consumer.go` - `SetV6Injector`, `injectorFor(fam)`, family-routed
      `InjectRoute`/`WithdrawRoute` (v4 -> inj, v6 -> injV6).
  → Constraint: the consumer already routes by family; the NSSA-vs-AS-wide decision belongs in the engine
    (`v6InjectExternal`), not the consumer.
- [ ] `internal/plugins/ospf/redist_wiring.go` - `InjectExternal`/`WithdrawExternal` v4/v6 branch.
- [ ] `internal/plugins/ospf/lsdb/origination.go` - `selfIsASBRLocked`/`SelfIsASBR` (resolved prerequisite).
- [ ] `internal/component/config/required.go` - the rule that a config with a `redistribute` block must
      carry a configured BGP reactor (Part B blocker #1). Read it; cite the exact requirement.
- [ ] `internal/component/config/redistribute/registry.go` - `RegisterSource`; registered sources are
      connected/kernel/ospf/isis. `static` is NOT a registered redistribution source (Part B blocker #2).
- [ ] `test/interop/scenarios/ospf-v6-redist-frr/` - the current PENDING scenario (ze.conf = plain v6 p2p;
      check.py = adjacency + skip-pass).
- [ ] `internal/plugins/ospfv3/packet/lsa_nssa.go` - v6 NSSA-LSA codec; confirm encode (WriteTo) exists or
      must be added (DecodeNSSALSA is present).

**Behavior to preserve:**
- OSPFv2 NSSA Type-7 origination + translation: byte/behavior-identical (ze-ospf-test 13/13,
  ospf-stub-nssa-frr passing).
- v6 Type-5 redistribution into non-NSSA areas: unchanged.
- The resolved `SelfIsASBR` E-bit behavior (TestOSPFASBRBitFromExternal / FromNSSAType7 / v6OriginateExternal).

**Behavior to change:**
- v6 redistribution must originate Type-7 (not Type-5) when the ASBR is attached to the target NSSA, and
  translate to Type-5 at the ABR for other areas.
- The v6 redist interop must install a route in FRR (no longer skip-pass).

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A redistributed IPv6 route (RIB / a registered redistribution source) enters via the redistribution
  producer -> `redistribute.Consumer.InjectRoute(prefix, ...)` (family = IPv6).

### Transformation Path
1. `Consumer.injectorFor(IPv6)` -> `injV6` -> `engine.InjectExternal` (`redist_wiring.go`) -> `v6InjectExternal`.
2. `v6InjectExternal` decides scope: **if** the engine is attached to an NSSA area (reuse `externalScope`)
   **and** policy says inject into it -> originate a **Type-7 (0x2007)** into that NSSA area store with the
   area's FA (P-bit per RFC 3101); **else** -> Type-5 AS-wide (current path, unchanged).
3. Router-LSA re-origination sets the E-bit via `SelfIsASBR` (already wired).
4. At the elected NSSA ABR, `translateNSSA` produces a **Type-5 (0x4005)** twin for the other areas
   (AF-aware encode), honoring the P=0 / FA=0 / self-Type-5 skips.
5. Flood (area scope for Type-7, AS scope for the translated Type-5) -> FRR installs an N-route (NSSA
   internal) or an E-route (other areas).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Redistribution framework ↔ OSPF engine | `Consumer` injector interface, family-routed | [ ] |
| Engine ↔ LSDB | AF-aware Type-7 origination + translation seam | [ ] |
| LSDB ↔ wire | v6 NSSA-LSA encoder (WriteTo) | [ ] |
| Engine ↔ FRR | interop: `show ipv6 ospf6 database`, route table | [ ] |

### Integration Points
- `externalScope` / `faByArea` (NSSA attachment + FA) - reuse for the v6 Type-7 FA.
- `OriginateNSSA`/`translateNSSA` - extend to be AF-aware (or add v6 variants behind the same policy).
- `SelfIsASBR` - already drives the E-bit AF-agnostically.

### Architectural Verification
- [ ] No bypassed layers (Type-7 scope decision in the engine, not the consumer)
- [ ] No unintended coupling (the redistribution framework stays AF-generic)
- [ ] No duplicated functionality (reuse the v4 NSSA policy; only the wire encode differs)
- [ ] Zero-copy preserved where applicable (buffer-first v6 NSSA-LSA encode)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A v6 NSSA-LSA wire encoder exists or is small to add (mirrors the v6 AS-External encode). | `ospfv3/packet/lsa_nssa.go` has DecodeNSSALSA; the AS-External LSA already has WriteTo. | Add a buffer-first WriteTo for the v6 NSSA-LSA. | read lsa_nssa.go; round-trip test | unvalidated |
| A-2 | `translateNSSA` can be made AF-aware (or given a v6 path) without changing v4 output. | nssa.go translateNSSA is v4-wire today. | Add a v6 translation path keyed on the engine codec. | v4 ze-ospf-test unchanged + v6 unit | unvalidated |
| A-3 | `externalScope`/`faByArea` returns the correct intra-NSSA FA for v6 interfaces. | externalScope is AF-shared; FA is an interface address (v6 link or global). | Compute the v6 FA from the v6 transport. | v6 unit + interop | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Part B: the redistribution framework requires a configured BGP reactor (`config/required.go`) and `static` is not a registered source, so a faithful interop needs a BGP peering. | Ze exits at config load (observed: empty `bgp {}` cascades). | Decide the approach (Key Design Decision); recommend a real BGP peering in the scenario, OR a separate framework follow-up to register `static` / decouple. |
| R-2 | v6 Type-7 -> Type-5 translation across areas (translator election + AF-aware encode) is the most intricate part. | translated Type-5 missing or malformed in FRR's other-area DB. | Stage it after intra-NSSA origination; reuse the v4 translator election verbatim, only swapping the encode. |
| R-3 | Setting the E-bit on an NSSA-area Router-LSA must match FRR's expectation (already proven for v4). | FRR "originating router is not an ASBR". | Already validated for v4 against FRR; v6 uses the same `SelfIsASBR`. |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `redistribute { destination ospf { import <src> } }` with an NSSA area + a v6 route | → | `v6InjectExternal` NSSA branch -> v6 Type-7 origination | `TestOSPFv6InjectExternalNSSAType7` (unit) + `ospf-v6-nssa-redist-frr` (interop) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | v6 route redistributed on an ASBR attached to NSSA area X | Originated as a Type-7 (0x2007) into area X's store (not Type-5 AS-wide) |
| AC-2 | The Type-7 from AC-1 | P-bit + forwarding address per RFC 3101 §2.3/2.4 (FA = the ASBR's intra-NSSA address; P clear unless translatable) |
| AC-3 | The v6 NSSA ASBR's Router-LSA | E-bit set (via `SelfIsASBR`); clears when the last Type-5/Type-7 is withdrawn |
| AC-4 | The elected NSSA ABR with a P-bit Type-7 | Translates it to a Type-5 (0x4005) for the other areas (RFC 3101 §3.6); P=0/FA=0/self-twin skipped |
| AC-5 | The redistributed v6 route is withdrawn | The Type-7 (and any translated Type-5) is MaxAge-purged; E-bit re-evaluated |
| AC-6 | v6 route redistributed into a non-NSSA area | Unchanged Type-5 AS-wide origination |
| AC-7 | FRR (NSSA internal) on the segment | Installs the redistributed v6 prefix as an N-route (and, across areas, an E-route) |
| AC-8 | Part B: the v6 redist interop scenario | Drives redistribution end-to-end and asserts the installed route (no skip-pass) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | redistributes a v6 route on an NSSA ASBR | source -> Consumer(v6) -> v6InjectExternal -> Type-7 into NSSA -> flood | `ospf-v6-nssa-redist-frr` |
| 2 | redistributes a v6 route on an NSSA ABR | as #1 + translateNSSA -> Type-5 to other areas | interop (multi-area) or unit |
| 3 | redistributes a v6 route on a normal-area ASBR | source -> Consumer(v6) -> v6InjectExternal -> Type-5 AS-wide | `ospf-v6-redist-frr` (Part B) |
| 4 | withdraws the route | WithdrawRoute(v6) -> v6WithdrawExternal / NSSA purge | unit + interop |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOSPFv6InjectExternalNSSAType7` | `internal/plugins/ospf/origination_v6_nssa_test.go` | NSSA-attached ASBR originates Type-7, not Type-5 | |
| `TestOSPFv6NSSAType7PbitFA` | same | P-bit + FA per RFC 3101 | |
| `TestOSPFv6TranslateNSSAToType5` | `internal/plugins/ospf/nssa_test.go` | v6 Type-7 -> Type-5 at the ABR; P=0/FA=0 skips | |
| `TestOSPFv6NSSAWithdrawPurges` | same | withdrawal purges Type-7 + translated Type-5 | |
| `TestNSSALSAv6RoundTrip` | `internal/plugins/ospfv3/packet/lsa_nssa_test.go` | v6 NSSA-LSA encode/decode round-trip (if encode added) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Type-7 prefix length | 0-128 | 128 | N/A | reject >128 |
| External metric | 0-0xFFFFFF | 0xFFFFFF | N/A | reject/clamp |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-ospf-v6-nssa-redist` | `test/parse/*.ci` (config) | NSSA + redistribute config parses + resolves | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-v6-nssa-redist-frr` | `test/interop/scenarios/` | FRR ospf6d | FRR (NSSA internal) installs Ze's redistributed v6 prefix as an N-route; no Type-5 leak into the NSSA | |
| `ospf-v6-redist-frr` (un-pend) | `test/interop/scenarios/` | FRR ospf6d | FRR installs Ze's redistributed v6 Type-5 route end-to-end (Part B) | |

### Future (if deferring any tests)
- Type-7 -> Type-5 cross-area translation interop may need a 3-router (ASBR-in-NSSA, ABR, backbone peer)
  topology; if deferred, validate translation by unit + in-process and document.

## Files to Modify
- `internal/plugins/ospf/origination_v6_external.go` - add the NSSA scope decision to `v6InjectExternal`/`v6WithdrawExternal`.
- `internal/plugins/ospf/nssa.go` - AF-aware `translateNSSA` (v6 path); reuse `externalScope`/`faByArea`.
- `internal/plugins/ospf/lsdb/nssa.go` - AF-aware `OriginateNSSA`/`PurgeNSSA` (or a v6 encoder seam) keeping the P-bit boundary enforcement.
- `internal/plugins/ospfv3/packet/lsa_nssa.go` - add a buffer-first `WriteTo` encoder if missing.
- `internal/plugins/ospf/redist_wiring.go` - no structural change expected (the scope decision lives in the engine).
- `test/interop/scenarios/ospf-v6-redist-frr/` - un-pend per the Part B decision.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No (redistribute + NSSA config already exist) | - |
| CLI commands/flags | No | - |
| Functional test for new behavior | Yes | `test/parse/*.ci`, interop |
| Doctor check | No (no new runtime dependency) | - |
| Prometheus counters | Reuse existing external/NSSA gauges | telemetry |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (OSPFv3 NSSA redistribution) |
| 2 | Config syntax changed? | No | - |
| 7 | Wire format changed? | Yes (v6 NSSA-LSA emission) | `docs/architecture/wire/*` if present |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc3101.md`, `rfc/short/rfc5340.md` |
| 10 | Test infrastructure changed? | Yes (new interop scenario) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Maybe | `docs/comparison.md` |
| 16 | Changed source referenced by doc anchors? | Check | grep `docs/` for the changed files |

## Files to Create
- `internal/plugins/ospf/origination_v6_nssa.go` - v6 Type-7 origination/withdrawal (mirrors origination_v6_external.go).
- `internal/plugins/ospf/origination_v6_nssa_test.go` - unit tests.
- `test/interop/scenarios/ospf-v6-nssa-redist-frr/{ze.conf,frr.conf,check.py}` - interop.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify/Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6-14 | per template |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add the NSSA branch stub in `v6InjectExternal` + a failing
   `TestOSPFv6InjectExternalNSSAType7`.
   - Verify: the branch is reached when the ASBR is NSSA-attached; test fails (stub originates nothing).
2. **Phase: v6 NSSA-LSA encode** — add `WriteTo` to the v6 NSSA-LSA if missing; round-trip test.
3. **Phase: v6 Type-7 origination** — originate the Type-7 into the NSSA area with the area FA + P-bit
   policy (reuse `externalScope`/`faByArea` + the v4 P-bit boundary rule). E-bit already via `SelfIsASBR`.
   - Verify: AC-1, AC-2, AC-3; v4 ze-ospf-test unchanged.
4. **Phase: v6 Type-7 -> Type-5 translation** — AF-aware `translateNSSA`; honor P=0/FA=0/self-twin skips.
   - Verify: AC-4, AC-5.
5. **Phase: Part B interop** — implement the chosen approach (BGP peering in the scenario) and un-pend
   `ospf-v6-redist-frr`; add `ospf-v6-nssa-redist-frr`.
   - Verify: AC-7, AC-8.
6. **Functional tests** → config parse + interop.
7. **RFC refs** → `// RFC 3101 §2.3/2.4/3.6`, `// RFC 5340` comments.
8. **Full verification** → `make ze-verify`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | Type-7 P-bit/FA per RFC 3101; translation skips P=0/FA=0/self-twin |
| Data flow | NSSA-vs-AS-wide decision in the engine, not the consumer; framework stays AF-generic |
| Rule: no-layering | v6 path reuses v4 policy; no parallel NSSA policy engine |
| v4 invariance | ze-ospf-test 13/13, ospf-stub-nssa-frr still pass |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| v6 Type-7 origination | `go test -run TestOSPFv6InjectExternalNSSAType7` |
| Translation | `go test -run TestOSPFv6TranslateNSSAToType5` |
| Interop installs route | `python3 test/interop/run.py ospf-v6-nssa-redist-frr` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | redistributed prefix length 0-128; metric bounds; FA validity |
| Resource | per-prefix LSID map bounded by redistributed route count (reuse redistV6 pattern) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| v4 ze-ospf-test regresses | Revert AF-aware change; isolate v6 path |
| FRR rejects Type-7 | Check P-bit/FA/E-bit against RFC 3101 + the v4 stub-nssa baseline |
| Part B config rejected | Re-decide framework approach (Key Design Decisions) |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Core Insight

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Part B interop approach: add a real BGP peering to the scenario | (a) register `static` as a redistribution source; (b) decouple `redistribute` from the BGP-reactor requirement | A BGP peering is the realistic redistribution deployment and keeps the framework intact; (a)/(b) are larger non-OSPF framework changes -- flag them as a SEPARATE framework follow-up rather than coupling them to OSPFv3. |
| Reuse v4 NSSA policy; vary only the wire encode | Build a parallel v6 NSSA engine | RFC 3101 NSSA semantics are AF-independent; only the LSA encoding differs (0x2007 + v6 prefixes) -- one policy, AF-aware encode. |

## Known Limitations
- Cross-area Type-7 -> Type-5 translation interop may require a 3-router topology; if not built, validate
  translation by unit/in-process and document.
- Part B keeps the redistribution-framework BGP coupling; registering non-BGP sources / decoupling is a
  separate follow-up (out of this spec's OSPFv3 scope).

## RFC Documentation

Add above enforcing code:
- `// RFC 3101 Section 2.3/2.4: "<P-bit / forwarding-address requirement>"`
- `// RFC 3101 Section 3.6: "<translation rule>"`
- `// RFC 5340 ...: "<v6 NSSA-LSA function code / prefix encoding>"`

## Implementation Summary

### What Was Implemented
- Added OSPFv3 NSSA Type-7 origination in the v6 redistribution path. `v6InjectExternal` now computes OSPFv3 external scope, originates NSSA-LSAs (`0x2007`) into attached NSSA areas, and originates AS-External-LSAs (`0x4005`) only when a normal/non-NSSA attachment allows Type 5.
- Added OSPFv3 Type-7 P-bit and forwarding-address handling at the origination boundary. The P-bit is encoded in prefix options, requires a non-zero IPv6 forwarding address, and is cleared when a local Type-5 twin exists.
- Extended NSSA translation to OSPFv3 by reusing the shared RFC 3101 translator election and source-preference policy while swapping the wire encode/decode path for v6 external/NSSA bodies.
- Added withdrawal handling for OSPFv3 Type-7 and translated Type-5 LSAs through the same self-LSA stale-flush mechanism used by v6 Type-5.
- Fixed the non-OSPF redistribution blocker for the v6 interop: BGP sources register at init, BGP RIB best-path changes emit generic redistribution events, `import bgp` matches `ibgp`/`ebgp` umbrella origins, and the OSPFv3 interop uses a real GoBGP peering as its route source.

### Bugs Found/Fixed
- `redistribute { destination ospf { import bgp } }` could parse before BGP registered `bgp`/`ibgp`/`ebgp` sources. `internal/component/bgp/redistribute/bgp.go` now registers them from `init`.
- The BGP best-path path did not feed the generic redistribution producer stream. `internal/component/bgp/redistribute/producer.go` and `internal/component/bgp/plugins/rib/rib_bestchange.go` now publish route-change events from best-path changes.
- Umbrella import rules rejected sub-sources such as `ebgp` when the rule was `import bgp`. `ImportRule.Accept` now allows either the exact source or the umbrella origin while still rejecting loops when `route.Origin == importingProtocol`.
- FRR rejected redistributed OSPFv3 routes until Ze completed the OSPFv3 database exchange and advertised a reachable Router-LSA. The Link-LSA/DD/LSReq follow-up in `spec-ospfv3-4-link-lsa` fixed the Loading -> Full drain instead of faking Router-LSA links.

### Documentation Updates
- Updated `docs/guide/ospf.md`, `docs/guide/configuration.md`, `docs/features.md`, `docs/architecture/core-design.md`, `docs/architecture/wire/ospfv3.md`, and `docs/research/ospf-implementation-guide.md` for unified OSPFv3, Link-LSAs, and v6 redistribution.
- Updated this spec and `plan/spec-ospf-af-unify.md` with implementation state and final-verification status.

### Deviations from Plan
- Part B used a real GoBGP peering instead of registering a fake static source or decoupling the redistribution framework from BGP. That kept the OSPFv3 interop faithful and left broader source-registration work out of scope.
- The implementation needed a BGP best-path producer bridge in addition to OSPF changes, because the interop source must be a real route producer.
- Cross-area Type-7 -> Type-5 interop remains unit/in-process validated in this pass; the end-to-end FRR scenario proves NSSA internal install and Type-5 non-leak in the NSSA.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| v6 Type-7 origination | done | `internal/plugins/ospf/origination_v6_nssa.go` | NSSA areas receive `0x2007` |
| v6 Type-5 normal-area path preserved | done | `internal/plugins/ospf/origination_v6_external.go` | Type-5 only when `canType5` |
| Type-7 translation | done | `internal/plugins/ospf/nssa.go` | shared translator policy with v6 encode |
| Withdrawal purge | done | `internal/plugins/ospf/origination_v6_external.go` | stale-flush keep set includes Type-7 and translations |
| Full v6 redist interop | done, final rerun pending | `test/interop/scenarios/ospf-v6-redist-frr/` | real GoBGP source feeds OSPFv3 |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestOSPFv6InjectExternalNSSAType7` | Type-7 in NSSA, no Type-5-only behavior |
| AC-2 | done | `TestOSPFv6NSSAType7PbitFA` | P-bit and FA rules |
| AC-3 | done | `TestOSPFv6OriginateRouterLSAABRNtBits` plus self-LSA tests | E/Nt flags preserved |
| AC-4 | done | `TestOSPFv6TranslateNSSAToType5` | v6 Type-7 -> Type-5 |
| AC-5 | done | `TestOSPFv6NSSAWithdrawPurges` | Type-7 and translated Type-5 purge |
| AC-6 | done | `TestOSPFv6InjectExternalNormalType5` | non-NSSA Type-5 unchanged |
| AC-7 | done, final rerun pending | `ospf-v6-nssa-redist-frr` session evidence | FRR installs NSSA route with no Type-5 leak |
| AC-8 | done, final rerun pending | `ospf-v6-redist-frr` session evidence | FRR installs redistributed v6 Type-5 route |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestOSPFv6InjectExternalNSSAType7` | pass | `internal/plugins/ospf/origination_v6_nssa_test.go` | Type-7 origination |
| `TestOSPFv6NSSAType7PbitFA` | pass | same | P-bit and FA |
| `TestOSPFv6TranslateNSSAToType5` | pass | same | translation |
| `TestOSPFv6NSSAWithdrawPurges` | pass | same | withdrawal |
| `TestOSPFv3NSSARoundTrip` | pass | `internal/plugins/ospfv3/packet/lsa_nssa_test.go` | v6 NSSA body codec |
| `TestBGPProducerBridgeEmitsRouteChange` | pass | `internal/component/bgp/redistribute/producer_test.go` | BGP producer bridge |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/ospf/origination_v6_external.go` | changed | Type-5/Type-7 scope decision and withdrawal |
| `internal/plugins/ospf/origination_v6_nssa.go` | created/changed | v6 Type-7 encoder boundary |
| `internal/plugins/ospf/nssa.go` | changed | v6 translation support |
| `internal/plugins/ospfv3/packet/lsa_nssa.go` | changed | NSSA body codec reuse |
| `test/interop/scenarios/ospf-v6-redist-frr/` | changed | real BGP source and route assertion |
| `test/interop/scenarios/ospf-v6-nssa-redist-frr/` | created/changed | NSSA route assertion and Type-5 leak check |
| `internal/component/bgp/redistribute/` | changed | source registration and best-change producer |
| `internal/component/config/redistribute/route.go` | changed | umbrella origin match with loop prevention |

### Audit Summary
- **Total items:** 8 ACs
- **Done:** 8 implemented
- **Partial:** final full verify and final interop rerun after docs remain pending
- **Skipped:** 0
- **Changed:** Part B required BGP redistribution producer wiring, not only OSPF files

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| OSPFv3 NSSA Type-7 redistribution | unit + interop | `TestOSPFv6InjectExternalNSSAType7`, `ospf-v6-nssa-redist-frr` session evidence |
| Full v6 redistribution interop | unit + interop | `TestBGPProducerBridgeEmitsRouteChange`, `ospf-v6-redist-frr` session evidence |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | | | | |

### Fixes applied
-

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end (interop)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
