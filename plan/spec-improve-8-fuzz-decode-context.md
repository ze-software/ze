# Spec: improve-8 -- Fuzz the Negotiated-Capability Decode Space

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

Anchor refresh (2026-07-22 plan review, design unchanged and implementable):
`ParseEVPN` drifted 187 -> 191 (`evpn/types.go`); all other cites verified
exact (`FuzzParseAttributes` asn4 hardcode `attrparse_fuzz_test.go`,
`FuzzParseNLRIs` `mpwire_test.go`, `capability.Parse`/
`ParseFromOptionalParams` `:177`/`:847` still fuzz-free).

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-improve-0-umbrella.md` -- set context and comparison-honesty scope
4. `tmp/session/session-state-improve-8-fuzz-decode-context-56997.md` -- research digest
5. `internal/core/bgp/context/context.go` -- EncodingContext

## Task

Ze's BGP wire-decode fuzzers hold the negotiated-capability context fixed or absent,
so the fuzzer never explores how negotiation state changes parsing. Verified
inventory (2026-07-10, research agent read every fuzz body): `FuzzParseAttributes`
hardcodes `asn4=true` (`internal/component/bgp/plugins/rib/storage/attrparse_fuzz_test.go,:30`),
leaving the 2-byte AS_PATH canonicalization branch (`attrparse.go`) never fuzzed;
`FuzzUnpackUpdate` passes a nil context (`internal/component/bgp/message/fuzz_test.go,:147,:154`);
the NLRI family targets (`mup`, `rtc`, `evpn`, `mvpn`, `vpls`, `flowspec`, `ls`)
parse with no add-path variation; and whole context-consuming surfaces have NO fuzz
target at all: OPEN capability parsing (`capability.Parse` `capability.go`,
`ParseFromOptionalParams` `:847`), `ParseMPReachNLRI` (`mpnlri.go`),
`ParseMPUnreachNLRI` (`mpnlri.go`).

The comparison-review daemon fuzzes negotiated-capability permutations by deriving
`Arbitrary` on its decode context (verified at primary source:
`fuzz/fuzz_targets/bgp/message_decode.rs:7-12`). Adopt the idea via the in-repo Go
idiom that already exists: TYPED FUZZ ARGUMENTS the engine mutates --
`FuzzParseNLRIs` already varies `hasAddPath` as a bool arg
(`internal/component/bgp/wireu/mpwire_test.go,:612`) and `FuzzParsePrefixes`
varies `addrSize` (`prefix_fuzz_test.go`). Widen existing targets over their
context dimensions and add targets for the uncovered context-consuming surfaces.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` - fuzz test conventions
  → Constraint: seed corpora are inline `f.Add(...)` with VALIDATES/PREVENTS/SECURITY doc comments (repo convention; no testdata/fuzz seed dirs exist)
- [ ] `ai/rules/repo-maintenance.md` - new targets must reach the fuzz enumeration
  → Constraint: `internal/le/fuzz/actions.go` enumerates targets individually (multi-target packages cannot use `-fuzz=.`); every new target MUST be added there or it never runs in `ze-fuzz-test`
- [ ] `ai/rules/performance.md` - decode-under-fuzz must respect wire package norms
  → Constraint: fuzz targets call producers as-is; no test-only decode wrappers that would diverge from production paths

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6793.md` (4-octet ASN) - the asn4 dimension being fuzzed
  → Constraint: AS_PATH canonicalization differs by negotiated asn4; both branches must be reachable by the fuzzer (verify summary exists at implementation; create if missing)
- [ ] `rfc/short/rfc7911.md` (ADD-PATH) - the add-path dimension
  → Constraint: path-id presence changes NLRI framing; both polarities must be fuzzed per family (verify summary exists at implementation; create if missing)

**Key insights:**
- In-repo precedent is typed fuzz args, NOT byte-prefix consumption: `FuzzParseNLRIs`
  (`mpwire_test.go`), `FuzzParsePrefixes` (`prefix_fuzz_test.go`),
  `FuzzChunkMPNLRI` (`message/fuzz_test.go,:238`). Follow it.
- Production context producer: `FromNegotiatedRecv`/`FromNegotiatedSend`
  (`internal/core/bgp/context/negotiated.go,:31`) -> `NewEncodingContext`
  (`context.go`, add-path map derivation :89-104), consumed at
  `reactor/peer.go,:490`. Context accessors: `ASN4()` :131, `ExtendedMessage()`
  :141, `MaxMessageSize()` :152, `AddPath(f)` :193.
- Decode consumers branching on context: `parseKnownAttribute` -> `fn(data,
  ctx.ASN4())` (`wire.go`); `ParseNLRIs(data, fam, hasAddPath)`
  (`mpwire.go`); extended-length checks (`context.go`).

## Current Behavior (MANDATORY)

**Source files read:** (research agent read each fuzz body; digest in tmp/session)
- [ ] `internal/component/bgp/plugins/rib/storage/attrparse_fuzz_test.go` - `FuzzParseAttributes` (:28) calls `ParseAttributes(data, true)` (:30): asn4 fixed true
- [ ] `internal/component/bgp/message/fuzz_test.go` - `FuzzParseHeader` :14, `FuzzUnpackOpen` :51, `FuzzUnpackUpdate` :117 (nil ctx :147,:154), `FuzzUnpackNotification` :164, `FuzzUnpackRouteRefresh` :202, `FuzzChunkMPNLRI` :231 (maxSize arg, add-path fixed false :247)
- [ ] `internal/component/bgp/plugins/nlri/vpn/types_fuzz_test.go` - two hardcoded targets: `FuzzParseVPN` :21 (addpath=false :41), `FuzzParseVPNAddPath` :61 (true :76) -- the two-target pattern predates typed-arg precedent
- [ ] `internal/component/bgp/wireu/mpwire_test.go` - `FuzzParseNLRIs` :603 varies `hasAddPath` :612 across 4 families :617-630 -- THE precedent
- [ ] `internal/core/bgp/capability/capability.go` - `Parse` :177, `ParseFromOptionalParams` :847: no Fuzz* in the package at all
- [ ] `internal/core/bgp/attribute` mpnlri - `ParseMPReachNLRI` (`mpnlri.go`), `ParseMPUnreachNLRI` (:532): not fuzzed
- [ ] `internal/le/fuzz/actions.go` - `ze-fuzz-test` runs each target at `-fuzztime=10s -timeout=60s`; `ze-fuzz-test-one FUZZ= PKG= TIME=` for one target

**Behavior to preserve:** (unless user explicitly said to change)
- Existing fuzz targets keep their names and seeds (corpus continuity); widening adds
  arguments via new targets or added args with re-seeded corpora, decided per target
  at implementation (renaming a target orphans its corpus).
- `ze-fuzz-test` runtime stays bounded: 10s per target unchanged.

**Behavior to change:** (only if user explicitly requested)
- None; test-only additions.

## Data Flow (MANDATORY)

### Entry Point
- `./le fuzz run` (all targets, enumerated in `internal/le/fuzz/actions.go`);
  `FUZZ=<target> PKG=<pkg> ./le fuzz run` for one.

### Transformation Path
1. Fuzz engine mutates (data []byte, context args: asn4 bool, hasAddPath bool, family index int, extended bool as applicable per target).
2. Target builds the decode call exactly as production does, passing context args to the same producer signatures (`ParseAttributes(data, asn4)`, `ParseNLRIs(data, fam, hasAddPath)`, ...).
3. The UPDATE-with-context target is DROPPED (see AC-3): no production entry decodes a full UPDATE against a negotiated context. The new capability and MP_REACH/MP_UNREACH targets decode from raw bytes only.
4. Crashes/panics surface via go test fuzzing; seeds pin both polarities of every varied dimension.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Fuzz engine ↔ decode producers | direct calls, production signatures | [ ] |
| Synthetic negotiation ↔ EncodingContext | production `NewEncodingContext` path only | [ ] |

### Integration Points
- `internal/le/fuzz/actions.go` target enumeration -- every new target added.
- Context construction (`internal/core/bgp/context/negotiated.go,:31` + `context.go`) is no longer an integration point: the UPDATE-with-context target that needed it is dropped (see AC-3).

### Architectural Verification
- [ ] No bypassed layers (targets call production decode producers directly)
- [ ] No unintended coupling (no test-only decode wrappers)
- [ ] No duplicated functionality (widen existing targets where they exist; new targets only for uncovered surfaces)
- [ ] Registration over hardcoding -- N/A (fuzz targets are ordinary _test.go discovered by go test; the make enumeration is the one hand-maintained list, updated per Constraint above)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An `EncodingContext` can be constructed in a fuzz target through the production producer without a live session | CONFIRMED by direct read 2026-07-10: `PeerIdentity{LocalASN, PeerASN, LocalRouterID, PeerRouterID}` (`capability/identity.go`) and `EncodingCaps{ASN4, ExtendedMessage, Families, AddPathMode}` (`capability/encoding.go`) are plain exported structs; `NewEncodingContext(&id, &caps, DirectionRecv)` (`context.go`) is the production entry (`negotiated.go` is a nil-guard wrapper over it) | - | phase-1 unit test TestEncodingContextFromFuzzArgs pins it | confirmed |
| A-2 | Typed-arg fuzzing explores the context space adequately (vs Holo's byte-derived Arbitrary) | Go's engine mutates typed args natively; polarity seeds pin both branches | Context combinations under-explored; switch the worst target to byte-prefix-derived context | Coverage check (`go test -fuzz -coverprofile` or fuzz -v beat lines) on `canonicalizeASPath` both branches during implementation | unvalidated |
| A-3 | Added targets keep `ze-fuzz-test` wall-clock acceptable | 10s per target; ~8 new targets = +80s | Trim per-target fuzztime for the new set or split a make tier | Time the target list after phase 2 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | New target added but not enumerated in internal/le/fuzz/actions.go -- never runs | target absent from `./le fuzz run` output | AC-4 asserts the enumeration; grep-based check in the same commit |
| R-2 | Renaming/widening an existing target orphans its accumulated corpus | corpus counters reset | prefer adding sibling targets over renaming; per-target decision recorded at implementation |
| R-3 | Fuzz-found decode crashes arrive as a flood once new surfaces open | multiple failures in first runs | each finding becomes a seed + fix per `ai/rules/completion.md`; findings are the point, not a risk to avoid -- budget review time |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ./le fuzz run | → | new/widened targets in the enumeration | enumeration includes every Fuzz* (AC-4 grep check) |
| ./le fuzz run FUZZ=FuzzParseCapabilities | → | capability.Parse under fuzz | FuzzParseCapabilities seed run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `FuzzParseAttributes` (or sibling target) | asn4 is a fuzz argument; seeds cover true AND false; `canonicalizeASPath` 2-byte branch (`attrparse.go`) reachable |
| AC-2 | EVPN NLRI target (the only listed family parser taking an add-path arg) | `FuzzParseEVPN` fuzzes `addpath` as an argument (or a sibling `*AddPath` target exists) with both-polarity seeds against `ParseEVPN(data, addpath)` (`internal/component/bgp/plugins/nlri/evpn/types.go`). Per-family add-path FRAMING is already covered upstream: `FuzzParseNLRIs` (`mpwire_test.go`) fuzzes `hasAddPath` over `ParseNLRIs(data, fam, hasAddPath)` (`mpwire.go`), which strips the path-id framing before the per-family parser. mup/rtc/mvpn/vpls/flowspec/ls parsers take no add-path arg, so nothing to widen there |
| AC-3 | Uncovered context-consuming surfaces | New targets exist: `capability.Parse`, `ParseFromOptionalParams`, `ParseMPReachNLRI`, `ParseMPUnreachNLRI`. NO whole-UPDATE-with-context target: `UnpackUpdate(data)` (`internal/component/bgp/message/update.go`) takes no context and `Update.Len(_ *EncodingContext)` (`update.go`) ignores its context arg, so no production entry decodes a full UPDATE against a negotiated context (a reconstructed one would be the test-only decode wrapper `buffer-first` forbids). Context-branching decode is covered by the widened `FuzzParseAttributes` (asn4, AC-1) plus the new MP_REACH/MP_UNREACH targets |
| AC-4 | `internal/le/fuzz/actions.go` | Enumeration lists every Fuzz* target in the affected packages; a grep check proves no orphan |
| AC-5 | Each varied dimension | Inline seeds pin both polarities with VALIDATES/PREVENTS comments per repo convention |

## Fuzz Target Matrix (added 2026-07-10 at design gate, per user request)

Every target this spec touches, with producer and fuzz-argument signature. "Widened"
= existing target gains args (as a sibling target if renaming would orphan corpus, R-2).

| Target | Kind | Producer (file:line) | Fuzz args | Seed requirement |
|--------|------|----------------------|-----------|------------------|
| FuzzParseAttributes | widened | `ParseAttributes` (`rib/storage/attrparse.go`) | data []byte, asn4 bool | existing seeds x both asn4 polarities |
| FuzzParseEVPN | widened (only listed family whose parser takes an add-path arg) | `ParseEVPN` (`plugins/nlri/evpn/types.go`) | data []byte, addpath bool | existing seeds x both polarities |
| mup/rtc/mvpn/vpls/flowspec/ls | not widened | family `Parse*` take no add-path arg; add-path FRAMING already fuzzed upstream by `FuzzParseNLRIs` over `ParseNLRIs(data, fam, hasAddPath)` (`mpwire.go`) | - | - |
| FuzzParseVPN + FuzzParseVPNAddPath | unchanged | `ParseVPN` | (two-target pattern already covers both) | keep |
| FuzzParseCapabilities | NEW | `capability.Parse` (`capability.go`) | data []byte | valid OPEN capability TLVs (multiprotocol, asn4, add-path, route-refresh) + truncations |
| FuzzParseFromOptionalParams | NEW | `ParseFromOptionalParams` (`capability.go`) | data []byte | optional-params blocks incl. RFC 9072 extended-length shape |
| FuzzParseMPReachNLRI | NEW | `ParseMPReachNLRI` (`mpnlri.go`) | data []byte, famIdx int, hasAddPath bool | per supported family, both polarities |
| FuzzParseMPUnreachNLRI | NEW | `ParseMPUnreachNLRI` (`mpnlri.go`) | data []byte, famIdx int, hasAddPath bool | per supported family, both polarities |

Context helper: the UPDATE-with-context target that would have consumed a
constructed `EncodingContext` is DROPPED (see AC-3). No production entry decodes a
full UPDATE against a negotiated context (`UnpackUpdate(data)` `update.go` takes no
context; `Update.Len(_ *EncodingContext)` `update.go` ignores it), and the
surviving new targets (`capability.Parse`, `ParseFromOptionalParams`,
`ParseMPReachNLRI(data)`, `ParseMPUnreachNLRI(data)`) each decode from raw bytes only
and take no context object. A-1 (that `NewEncodingContext` can be built in a target
via the production path) and `TestEncodingContextFromFuzzArgs` are retained only as
feasibility reference; they are no longer tied to a shipping target.

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Developer runs `./le fuzz run` | enumeration -> all targets incl. new surfaces at 10s each | AC-4 grep + target run |
| 2 | Fuzzer finds a decode crash under asn4=false | failing input minimized -> seed + fix | regression seed committed with the fix |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestEncodingContextFromFuzzArgs | context package or target-side helper test | A-1: production-path context construction from plain args | |
| FuzzParseCapabilities (+seeds) | `internal/core/bgp/capability/` fuzz test | AC-3 | |
| FuzzParseMPReachNLRI / FuzzParseMPUnreachNLRI (+seeds) | attribute package fuzz test | AC-3 | |
| widened FuzzParseAttributes (asn4 arg) | attrparse fuzz test | AC-1 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| (none numeric; N/A -- fuzz inputs are engine-generated) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A -- fuzz targets are unit-level by nature; `./le fuzz run` is the executable gate (functional-test-gate: no user-facing behavior changes) | - | - | |

### Interop Tests (MANDATORY for protocol features)
- N/A: no wire behavior change; fuzzing exercises existing decode paths.

## Files to Modify
- `internal/component/bgp/plugins/rib/storage/attrparse_fuzz_test.go` - asn4 as fuzz arg (AC-1)
- `internal/component/bgp/plugins/nlri/evpn/types_fuzz_test.go` - add-path dimension for `FuzzParseEVPN` (AC-2); mup/rtc/mvpn/vpls/flowspec/ls unchanged (no add-path arg; framing covered by `FuzzParseNLRIs`)
- `internal/le/fuzz/actions.go` - enumerate new targets (AC-4)

## Files to Create
- `internal/core/bgp/capability/capability_fuzz_test.go` - FuzzParseCapabilities, FuzzParseFromOptionalParams (AC-3)
- attribute package fuzz file for MP_REACH/MP_UNREACH (AC-3; exact file beside `mpnlri.go` at implementation)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | test-only |
| YANG validation constraints | N/A | none |
| YANG custom validators | N/A | none |
| CLI commands/flags | N/A | none |
| CLI grammar (action before identifier) | N/A | none |
| Editor autocomplete | N/A | none |
| Functional test for new RPC/API | N/A | no new RPC/API |
| Pipe completeness | N/A | none |
| Env var registration | N/A | none |
| Doctor check for runtime dependencies | N/A | none |
| Prometheus counters/metrics | N/A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | test-only |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | none |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | decode robustness, not support-level change |
| 10 | Test infrastructure changed? | Yes | fuzz-target documentation where `internal/le/fuzz/actions.go` conventions live (named at implementation per discovery-updates) |
| 11 | Affects daemon comparison? | No | fuzz depth is not a comparison row |
| 12 | Internal architecture changed? | No | none |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | none |
| 16 | Any changed source file is referenced by existing doc source anchors? | Check at implementation | grep `docs/` for anchors |
| 17 | Existing docs show config/CLI/API examples for this area? | No | none |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - A-1 context-construction unit test; new empty
   fuzz targets registered in internal/le/fuzz/actions.go enumeration (AC-4 grep check red)
2. **Phase: widen existing targets** - asn4 arg (AC-1), add-path per family (AC-2),
   both-polarity seeds (AC-5); per-target rename-vs-sibling decision recorded (R-2)
3. **Phase: new surfaces** - capability, MP_REACH/MP_UNREACH (AC-3); no UPDATE-with-context target (dropped -- see AC-3)
4. **Phase: soak + coverage check** - A-2/A-3 measurements recorded in this spec;
   findings triaged per R-3
5. `./le verify current mode full`, learned summary, two-commit closure

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-5 with file:line |
| Correctness | targets call production producers with production signatures; no wrappers |
| Rule: no-workarounds | fuzz findings fixed at the decode source, never by weakening the target |
| Enumeration | every Fuzz* in affected packages present in internal/le/fuzz/actions.go (AC-4) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Widened + new fuzz targets | `rg 'func Fuzz' <packages>` matches internal/le/fuzz/actions.go enumeration |
| Both-polarity seeds | `rg 'f.Add' <fuzz files>` shows true/false pairs per varied arg |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | this spec IS the input-validation hardening; findings get seeds + fixes |
| Resource exhaustion | fuzztime/timeout caps unchanged (10s/60s) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Fuzz finding (decode crash) | Fix at source + regression seed (R-3); never weaken the target |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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
- Umbrella finding 9 (CI fuzz buildability) was DECLINED and stays declined: Go
  compiles fuzz targets in every `go test` run. This spec is input-space widening,
  a different property, verified against the reviewed daemon's actual mechanism
  (`Arbitrary`-derived DecodeCxt, `fuzz/fuzz_targets/bgp/message_decode.rs:7-12`).
- Sibling spec `plan/spec-fixit-parser-fuzz-gaps.md` (BMP receiver TLV / RADIUS VSA /
  DHCP server packet) is COMPLEMENTARY, not a conflict: disjoint packages and disjoint
  fuzz-target names, so no `internal/le/fuzz/actions.go` target collision. The only shared touch is
  the `internal/le/fuzz/actions.go` enumeration and the `docs/functional-tests.md` fuzz-target COUNT
  (already stale). If both specs land, that count must SUM both specs' additions.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Typed fuzz arguments for context dimensions | byte-prefix-derived context (Holo's Arbitrary shape) | in-repo precedent (`FuzzParseNLRIs` `mpwire_test.go`); Go's engine mutates typed args natively; byte-prefix kept as A-2 fallback |
| Context built through the production producer | hand-rolled context literals in targets | a hand-rolled context can drift from `NewEncodingContext` derivation (add-path map :89-104); production path keeps fuzz honest |
| Sibling targets preferred over renames | rename existing targets to add args | renames orphan accumulated corpora (R-2) |

## Known Limitations
- Fuzz explores decode robustness, not semantic correctness of negotiated behavior;
  interop and conformance suites own semantics.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Fuzzer explores negotiated-capability space | coverage evidence on both asn4 branches (A-2 check) | (fill during implementation) |
| No context-consuming decode surface unfuzzed | AC-3/AC-4 grep evidence | (fill during implementation) |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block -- record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

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
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `./le verify current mode full` passes (lint + all ze tests)
- [ ] Feature code integrated (fuzz targets + make enumeration)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (rfc/short refs on the two fuzzed dimensions)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior (N/A with justification above)
- [ ] Interop tests for protocol features (N/A with justification above)
- [ ] Goal Validation table filled with concrete evidence
