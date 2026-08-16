# Spec: ipsec-lifetime-volume

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

Anchor refresh (2026-07-22 plan review, design still valid, feature not
landed): every `rekey.go` cite was off by ~+200 lines (the file grew at the
top); the citations below are now updated in-body -- `softBytes`/`byteCount`
`rekey.go`, `newLifetimeState` `:226`, `softExpired` byte check
`:255-262`. `ESPGroup` (`types.go`) and `leaf lifetime`
(`ze-ipsec-conf.yang`) still exact.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/ike/ipsec/types.go` - `ESPGroup` (Lifetime only)
4. `internal/component/ike/engine/rekey.go` - `lifetimeState` (dead byte-expiry scaffolding)
5. `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - esp-group lifetime leaf

## Task

Ze rekeys ESP (child) SAs only on a time-based lifetime. It cannot rekey by volume,
a byte count (`life-bytes`) or packet count (`life-packets`). High-throughput
tunnels reach cryptographic volume limits long before a time lifetime expires;
without volume-based rekey they either rekey too rarely (weakening security) or must
use an artificially short time lifetime (needless churn).

Add ESP-group volume-based rekey: `life-bytes` and/or `life-packets` leaves that
trigger a child-SA rekey when the SA has processed the configured volume, in
addition to (whichever comes first) the time lifetime. The rekey engine already has
byte-expiry scaffolding that is currently never fed by config; this wires it.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - IKE engine and SA lifecycle.
  → Constraint: reuse the existing `lifetimeState.softExpired` byte path; do not add a parallel timer system.
- [ ] `ai/rules/config.md` / `ai/rules/config.md` - the new leaves.
  → Constraint: a byte count can far exceed the uint32 maximum, so the leaf type and Go field MUST be 64-bit.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 7296 - SA lifetime may be expressed as time and/or volume; rekey via CREATE_CHILD_SA.
  → Constraint: volume-based rekey triggers the same child-SA rekey path as time-based; only the trigger differs.

**Key insights:**
- `lifetimeState` already carries `softBytes`/`byteCount` and checks them in `softExpired`; the gap is that `newLifetimeState` never sets `softBytes` from config, and no byte counter is fed from the dataplane.
- Byte volumes reach ~10^13, which overflows uint32; the leaf and field must be uint64 from the outset.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/ipsec/types.go` - `ESPGroup{ Name; Lifetime uint32; PFS; Proposals }` (types.go): the only lifetime is time in seconds; no byte/packet field. `IKEGroup` similarly (types.go).
- [ ] `internal/component/ike/engine/rekey.go` - `lifetimeState` has `softBytes uint64` / `byteCount uint64` (rekey.go) and `softExpired` returns true when `softBytes > 0 && byteCount >= softBytes` (rekey.go), but `newLifetimeState(lifetimeSec uint32)` only ever sets the time fields (rekey.go+) — the byte path is dead.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - esp-group `leaf lifetime { type uint32 { range "0..86400"; } }` (ze-ipsec-conf.yang); no `life-bytes`/`life-packets` leaves.
- [ ] `internal/component/ike/ipsec/config.go` - esp-group parser caps lifetime at 86400 (config.go,245); no volume parsing.

**Behavior to preserve:**
- Time-based rekey behaviour is unchanged when no volume leaves are set.
- The child-SA rekey mechanism (CREATE_CHILD_SA path) is reused as-is.
- Existing esp-group configs continue to work.

**Behavior to change:**
- `life-bytes`/`life-packets` config feed the rekey trigger; a byte/packet counter drives `softExpired`.

## Data Flow (MANDATORY)

### Entry Point
- Config: `life-bytes` and `life-packets` leaves in the esp-group container of `ze-ipsec-conf.yang` (uint64 ranges).

### Transformation Path
1. Parser reads `life-bytes`/`life-packets` into new uint64 fields on `ESPGroup`.
2. `newLifetimeState` is extended to set `softBytes` (and a new `softPackets`) from the esp-group config, in addition to the time fields.
3. A per-SA byte/packet counter is fed from processed traffic (dataplane/SA stats) into `lifetimeState.byteCount` (and packet count).
4. `softExpired` triggers when time OR byte OR packet soft limit is reached; the existing child-SA rekey runs.
5. Hard limits (if modelled) tear down as the time hard-limit does today.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ ESPGroup | `life-bytes`/`life-packets` → uint64 fields | [ ] |
| Config ↔ engine | `newLifetimeState` sets `softBytes`/`softPackets` | [ ] |
| Dataplane ↔ lifetimeState | processed byte/packet count fed to the counter | [ ] |

### Integration Points
- `ESPGroup` (`types.go`) - add `LifeBytes uint64` / `LifePackets uint64`.
- `newLifetimeState` (`rekey.go+`) - populate `softBytes`/`softPackets`.
- SA stats source - where processed byte/packet counts are read to feed the counter.

### Architectural Verification
- [ ] No bypassed layers (config → ESPGroup → lifetimeState)
- [ ] No unintended coupling (reuse existing rekey; add trigger inputs)
- [ ] No duplicated functionality (the byte path already exists; feed it)
- [ ] Registration over hardcoding — new fields on `ESPGroup`; engine-driven, no central switch.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A per-SA processed byte/packet count is available to feed the counter | rekey.go references byteCount | need to source stats from the dataplane | trace where SA byte stats live during RESEARCH | unvalidated |
| A-2 | uint64 is sufficient and correct for the ranges | byte volumes reach ~10^13 | overflow if 32-bit | boundary test at range max | unvalidated |
| A-3 | Packet-count path can mirror the byte path in `lifetimeState` | symmetry with softBytes | packets need separate handling | add `softPackets`/`packetCount` during design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Declaring the leaf/field as 32-bit silently caps large volumes | large life-bytes truncates | uint64 leaf + field from the start; boundary test at max |
| R-2 | No live byte counter → volume rekey never fires | volume limit ignored at runtime | confirm the SA stats source before claiming done (wiring test) |
| R-3 | Frequent rekey under high throughput | rekey storms | sane minimum on the volume leaves |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| esp-group `life-bytes N` | → | `newLifetimeState` sets `softBytes`; counter triggers rekey | `test/plugin/ipsec-life-bytes.ci` |
| SA processes ≥ N bytes | → | child-SA rekey fires | `test/plugin/ipsec-life-bytes.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `life-bytes N`, SA processes ≥ N bytes | child SA rekeys (before time lifetime) |
| AC-2 | `life-packets P`, SA processes ≥ P packets | child SA rekeys |
| AC-3 | both time and volume set | whichever soft limit is reached first triggers rekey |
| AC-4 | `life-bytes` near uint64 max | accepted, not truncated |
| AC-5 | no volume leaves | time-only behaviour unchanged |
| AC-6 | volume below configured minimum | config verify rejects |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | sets a byte lifetime on a high-throughput tunnel | config → ESPGroup.LifeBytes → softBytes → counter → rekey | `test/plugin/ipsec-life-bytes.ci` + interop |
| 2 | leaves volume unset | time-based rekey unchanged | `test/plugin/ipsec-life-bytes.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLifetimeStateSoftBytes` | `internal/component/ike/engine/rekey_test.go` | byte soft-expiry triggers | |
| `TestLifetimeStateSoftPackets` | `internal/component/ike/engine/rekey_test.go` | packet soft-expiry triggers | |
| `TestLifetimeFirstLimitWins` | `internal/component/ike/engine/rekey_test.go` | earliest of time/byte/packet triggers | |
| `TestESPGroupVolumeParse` | `internal/component/ike/ipsec/config_test.go` | uint64 life-bytes/packets parsed, no truncation | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| life-bytes | design min .. ~2.7e13 (uint64) | range max | below min | above declared max |
| life-packets | design min .. large (uint64) | range max | below min | above declared max |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-life-bytes` | `test/plugin/ipsec-life-bytes.ci` | tunnel rekeys after configured volume | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-ipsec-life-bytes-peer` | `test/interop/scenarios/` | strongSwan | volume-triggered rekey interoperates; peer sees a CREATE_CHILD_SA rekey | |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/ike/ipsec/types.go` - add `LifeBytes`/`LifePackets uint64` to `ESPGroup`
- `internal/component/ike/ipsec/config.go` - parse the new leaves (uint64)
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - add `life-bytes`/`life-packets` (uint64 range)
- `internal/component/ike/engine/rekey.go` - set `softBytes`/`softPackets` in `newLifetimeState`; add packet counter
- SA stats source - feed processed byte/packet counts into `lifetimeState`

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | esp-group `life-bytes`/`life-packets`; `ai/rules/config.md` |
| YANG validation constraints | [ ] yes | uint64 `range`; sane minimum |
| Functional test for new behaviour | [ ] yes | `test/plugin/ipsec-life-bytes.ci` |
| Prometheus counters/metrics | [ ] maybe | rekey-by-volume counter per SA |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |
| 9 | RFC behavior implemented? | [ ] yes | RFC 7296 lifetime summary |

## Files to Create
- `test/plugin/ipsec-life-bytes.ci` - functional test
- (unit tests extend `rekey_test.go`, `config_test.go`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add uint64 `life-bytes`/`life-packets` YANG + `ESPGroup` fields; set `softBytes` in `newLifetimeState`; failing `test/plugin/ipsec-life-bytes.ci`.
2. **Phase: Counter feed** — source per-SA processed byte/packet counts and feed `lifetimeState`.
   - Tests: `TestLifetimeStateSoftBytes`, `TestLifetimeStateSoftPackets`
3. **Phase: First-limit-wins + packet path** — ensure earliest of time/byte/packet triggers; add `softPackets`.
   - Tests: `TestLifetimeFirstLimitWins`, `TestESPGroupVolumeParse`
4. **Phase: Validation** — uint64 range + minimum.
5. **Functional + interop (strongSwan) tests**
6. **Full verification** → `make ze-precommit-verify`
7. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | uint64 end-to-end (no 32-bit truncation); first-limit-wins |
| Data flow | a real byte/packet counter feeds the trigger (not just config) |
| Rule: no-workarounds | volume rekey actually fires at runtime, proven by wiring test |
| Registration over hardcoding | fields on ESPGroup; engine-driven |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| volume rekey | `go test ./internal/component/ike/engine -run Lifetime` |
| no truncation | boundary test at uint64 max passes |
| interop | `NN-ipsec-life-bytes-peer` against strongSwan |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Correctness | volume limits enforce crypto rekey before overuse |
| Input validation | volume leaves bounded; no zero/tiny storm |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## RFC Documentation
Add `// RFC 7296` comments where the volume soft-limit triggers the child-SA rekey,
noting time/volume are alternative expressions of the same lifetime.

## Implementation Summary
### What Was Implemented
- (fill during implementation)

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
