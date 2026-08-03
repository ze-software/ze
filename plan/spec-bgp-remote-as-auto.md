# Spec: bgp-remote-as-auto

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

Anchor refresh (2026-07-22 plan review, design unchanged): resolver anchors
had drifted; citations below updated in-body -- `resolveDynamicPeerSettings`
`reactor_dynamic.go` with `PeerAS = remoteAS` at `:347`; `PeerAS: 0` at
`:117`. The config gate (`config.go`) and `leaf remote`
(`ze-bgp-conf.yang`, no `auto`) still hold.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/bgp/reactor/config.go` - static-peer config requirement
4. `internal/component/bgp/reactor/reactor_dynamic.go` - learn-AS-from-OPEN machinery
5. `internal/component/bgp/yang/ze-bgp-conf.yang` - `session/asn/remote` leaf

## Task

For a statically-configured BGP peer, Ze requires the peer AS to be a fixed number:
config load rejects a peer whose `session > asn > remote` is absent. There is no way
to say "accept whatever AS this peer announces in its OPEN". The machinery to learn a
peer's AS from its OPEN already exists, but only for dynamic (listen-range) peers.

Add an `auto` mode (and the `internal` / `external` constraint modes) to a static
peer's remote AS:
- `auto`: leave the peer AS unset and adopt the AS carried in the received OPEN.
- `internal`: adopt the learned AS but require it to equal the local AS (iBGP only).
- `external`: adopt the learned AS but require it to differ from the local AS (eBGP only).

This reuses the existing dynamic-peer AS-resolution path so a statically-addressed peer
can be brought up without pinning its AS in advance.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - how dynamic peers learn AS from OPEN.
  → Constraint: `auto` must reuse this resolution, not add a second AS-learning path.
- [ ] `ai/rules/config.md`, `ai/rules/config.md` - the remote-as mode option.
  → Constraint: `remote` stays an ASN; the mode is expressed without breaking the existing numeric leaf.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - BGP-4 OPEN message and the My Autonomous System field.

**Key insights:**
- The learn-from-OPEN resolver already exists (`resolveDynamicPeerSettings`); the feature wires it to static peers.
- Ze does not today validate the OPEN's AS against the configured remote-as, so `auto` mainly relaxes the config-load requirement and then binds the learned AS.
- iBGP vs eBGP is decided by `LocalAS == PeerAS`, so `internal`/`external` are simple post-learn constraints.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/config.go` - static-peer load rejects a missing remote AS: `if peerAS == 0 { return ...missing required session > asn > remote }` (config.go). This is the exact gate `auto` must relax.
- [ ] `internal/component/bgp/reactor/reactor_dynamic.go` - dynamic peers set `PeerAS: 0, // Learned from OPEN` (reactor_dynamic.go) and `resolveDynamicPeerSettings` (reactor_dynamic.go) sets `p.settings.PeerAS = remoteAS` from `open.ASN4`/`open.MyAS` (reactor_dynamic.go). This is the reusable resolver.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `leaf remote { type zt:asn; }` under `session > asn` (ze-bgp-conf.yang); not `mandatory` in YANG, so the requirement lives only in Go.

**Behavior to preserve:**
- A static peer with a numeric `remote` behaves exactly as today.
- Dynamic (listen-range) peers keep learning their AS unchanged.
- iBGP/eBGP classification (`LocalAS == PeerAS`) and AS_PATH handling remain correct once the AS is known.

**Behavior to change:**
- A static peer may set `remote` to `auto` / `internal` / `external`; the config load no longer requires a numeric AS in that case, and the AS is bound from the OPEN.

## Data Flow (MANDATORY)

### Entry Point
- Config: `session > asn > remote` set to a mode token (`auto` | `internal` | `external`) instead of a number.

### Transformation Path
1. Config parse records the remote-as mode; for a mode token the numeric peer AS is left 0 and the load-time requirement is skipped.
2. The peer starts passively (or actively) with PeerAS unset, like a dynamic peer.
3. On receiving the OPEN, the learned AS is resolved from `open.ASN4`/`open.MyAS` via the existing resolver and stored as `PeerAS`.
4. For `internal`, the session is rejected unless learned AS == local AS; for `external`, rejected unless learned AS != local AS; for `auto`, accepted either way.
5. Downstream iBGP/eBGP handling proceeds using the now-known PeerAS.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ reactor | remote-as mode parsed; numeric requirement conditional | [ ] |
| OPEN ↔ settings | learned AS bound via the shared resolver | [ ] |
| Settings ↔ session admit | internal/external constraint enforced post-learn | [ ] |

### Integration Points
- `internal/component/bgp/yang/ze-bgp-conf.yang` - allow `auto`/`internal`/`external` for the remote-as mode.
- `internal/component/bgp/reactor/config.go` - skip the numeric requirement for a mode token; record the mode.
- `internal/component/bgp/reactor/reactor_dynamic.go` / peer settings - reuse `resolveDynamicPeerSettings`-style learn for static peers.
- Session admission - apply the internal/external constraint after the AS is learned.

### Architectural Verification
- [ ] No bypassed layers (AS learned through the existing resolver)
- [ ] No unintended coupling (mode is a peer setting; no core struct change)
- [ ] No duplicated functionality (single learn-from-OPEN resolver for dynamic and auto peers)
- [ ] Registration over hardcoding - the mode is a config-driven peer setting; no per-mode branch is hardcoded into a core/shared package beyond the reactor.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The dynamic resolver can be reused for statically-addressed peers | reactor_dynamic.go,347 | need a separate learn path | trace the resolver's inputs during audit | unvalidated |
| A-2 | The remote-as leaf can carry a mode token without breaking the numeric type | ze-bgp-conf.yang | needs a separate mode leaf | prototype the union/enum in YANG | unvalidated |
| A-3 | Nothing downstream reads PeerAS before the OPEN is processed | config.go static path sets it early today | early readers see 0 | grep PeerAS readers on the connect path | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A downstream consumer assumes PeerAS is known at config time | nil/zero AS panic or misclassification pre-OPEN | defer AS-dependent setup until after learn, as dynamic peers already do |
| R-2 | `internal`/`external` constraint applied too late (after routes exchanged) | wrong-AS peer briefly established | enforce the constraint at session admission, before Established |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| static peer `remote auto`, peer sends OPEN AS 65010 | → | PeerAS resolved to 65010 from OPEN | `test/ci/bgp-remote-as-auto.ci` |
| static peer `remote internal`, peer AS != local | → | session rejected | `test/ci/bgp-remote-as-auto.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | static peer `remote auto`, OPEN carries AS X | PeerAS becomes X; session establishes |
| AC-2 | static peer `remote auto` | config load no longer requires a numeric remote AS |
| AC-3 | numeric `remote 65001` | unchanged behaviour |
| AC-4 | `remote internal`, learned AS == local | establishes (iBGP) |
| AC-5 | `remote internal`, learned AS != local | rejected |
| AC-6 | `remote external`, learned AS != local | establishes (eBGP) |
| AC-7 | `remote external`, learned AS == local | rejected |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a peer without knowing its AS in advance | `remote auto` → learn from OPEN → establish | `test/ci/bgp-remote-as-auto.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRemoteAsAutoSkipsRequirement` | `internal/component/bgp/reactor/config_test.go` | `auto` peer loads without a numeric AS | |
| `TestRemoteAsAutoLearnsFromOpen` | `internal/component/bgp/reactor/reactor_dynamic_test.go` | learned AS bound from OPEN | |
| `TestRemoteAsInternalExternalConstraint` | `internal/component/bgp/reactor/..._test.go` | internal/external admit/reject | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| learned ASN | 1..4294967295 | 4294967295 | 0 (no AS in OPEN) | 4294967296 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-remote-as-auto` | `test/ci/bgp-remote-as-auto.ci` | auto peer learns AS; internal/external constraints enforced | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| auto peer accepts a real neighbour's announced AS | `test/ci/` | GoBGP | learn-from-OPEN interops with a standard speaker | |

### Future (if deferring any tests)
- None planned.

## RFC Documentation
- RFC 4271 (`rfc/short/rfc4271.md`): the OPEN message and My Autonomous System field that the learned AS is taken from.

## Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - allow `auto`/`internal`/`external` for the remote-as mode
- `internal/component/bgp/reactor/config.go` - skip the numeric requirement for a mode token; record the mode
- `internal/component/bgp/reactor/reactor_dynamic.go` - reuse the learn-from-OPEN resolver for static auto peers
- `internal/component/bgp/reactor/` (session settings/admission) - enforce internal/external post-learn

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (mode value) | [ ] yes | `ze-bgp-conf.yang` remote-as; `ai/rules/config.md` |
| Functional test | [ ] yes | `test/ci/bgp-remote-as-auto.ci` |
| Interop test | [ ] yes | GoBGP neighbour |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `test/ci/bgp-remote-as-auto.ci` - functional + interop test
- (unit tests extend existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - accept the `auto`/`internal`/`external` mode in YANG + config (parsed, unused); failing `test/ci/bgp-remote-as-auto.ci`.
2. **Phase: Relax requirement + learn** - skip the numeric requirement for a mode token; bind the learned AS via the existing resolver.
   - Tests: `TestRemoteAsAutoSkipsRequirement`, `TestRemoteAsAutoLearnsFromOpen`
3. **Phase: Constraints** - enforce internal/external at session admission.
   - Tests: `TestRemoteAsInternalExternalConstraint`
4. **Functional + interop test (GoBGP)**
5. **Full verification** → `make ze-verify`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | numeric peers unchanged; learned AS bound before AS-dependent setup; constraints enforced pre-Established |
| Reuse | single learn-from-OPEN resolver shared with dynamic peers |
| Registration over hardcoding | mode is a config-driven setting; no per-mode core switch |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| requirement relaxed | `go test ./internal/component/bgp/reactor -run RemoteAsAuto` |
| constraints | `go test ./internal/component/bgp/reactor -run InternalExternal` |
| interop | `test/ci/bgp-remote-as-auto.ci` vs GoBGP |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Admission | `internal`/`external` constraint enforced before the session reaches Established |
| No unintended openness | `auto` on a static peer still matches by configured IP, so it does not accept arbitrary neighbours |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (learned ASN)
- [ ] Functional + interop tests for end-to-end behavior
