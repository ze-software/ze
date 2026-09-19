# Spec: ike-reauth

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/ike/engine/established.go` - SA lifetime maintenance loop
4. `internal/component/ike/engine/rekey.go` - `rekeyIKESA`, lifetime state
5. `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `lifetime` leaf (current rekey semantics)

## Task

Ze's IKE SA lifetime currently drives a **rekey** only: on soft expiry the engine
rekeys the IKE SA (deriving new keys from the old, preserving the authenticated
identity). Scheduled reauthentication requires a fresh IKE_SA_INIT + IKE_AUTH
that re-runs authentication. The current `lifetime` leaf already describes
rekey; its earlier "0 disables reauth" wording has been corrected.

Add IKEv2 reauthentication:

- A per-connection (ike-group, with per-peer override) knob to enable reauth and
  set a reauth time.
- On reauth-time expiry, start a fresh authenticated IKE SA and negotiate its
  Child SAs while the old SAs still carry traffic. Switch to the replacement
  before deleting the old SAs.
- Introduce a correctly named reauth control and preserve the current
  `lifetime` leaf's rekey meaning.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-3-data-model.md` - the typed model between the YANG schema and the IKE engine
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - the native IKEv2 state machine above the wire codec and the crypto layer
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - ESP Child SA creation after IKE_AUTH, and the dataplane abstraction
- [ ] `docs/architecture/core-design.md` - IKE component placement, reconnect loop
  → Constraint: reuse the full-handshake machinery with explicit old/new SA ownership. The failure-recovery loop's teardown-first ordering cannot supply make-before-break reauthentication.

### RFC Summaries (MUST for protocol work)
- [ ] RFC 7296 §2.8 (rekeying) and §2.8.3 (reauthentication vs rekey).
  → Constraint: reauth creates a new IKE SA through fresh IKE_SA_INIT/IKE_AUTH and new Child SAs. This design requires installation before old-SA deletion, as R-1 and the critical review checklist already require; it is separate from CREATE_CHILD_SA rekey.

**Key insights:**
- The existing initiator handshake is reused; concurrent replacement and old-SA ownership need design before implementation.
- The existing `lifetime` semantics (rekey) must remain the default; reauth is opt-in.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/established.go` - the maintenance loop: on IKE SA soft expiry it rekeys via `rekeyIKESA` (established.go, "RFC 7296 Section 1.3.3: IKE SA rekey via CREATE_CHILD_SA"); on hard expiry it tears down and returns `errTimeout` (established.go).
- [ ] `internal/component/ike/engine/rekey.go` - `rekeyIKESA` derives new keys from the old SK_d and copies the authenticated identity into the new SA (rekey path, not reauth); `newLifetimeState` builds the single soft/hard lifetime timer.
- [ ] `internal/component/ike/ipsec/types.go` - `IKEGroup` carries only `Lifetime uint32`; no `Reauth`/`ReauthTime` field.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - the IKE `lifetime` leaf defaults to 28800 and describes rekey; 0 starts no rekey. The original wording defect is already corrected. No reauth control is claimed implemented here.
- [ ] `internal/component/ike/engine/fsm.go` / `reconcile.go` - `runInitiator` performs the full IKE_SA_INIT+IKE_AUTH; the reconnect loop re-runs it on failure with backoff.

**Behavior to preserve:**
- Default lifetime behaviour stays a rekey (`rekeyIKESA`); existing tunnels are unaffected when reauth is not configured.
- Child SA (ESP) lifetime handling is unchanged.
- The full initiator handshake (`runInitiator`) is reused, not duplicated.

**Behavior to change:**
- Add a reauth timer that starts a fresh authentication, installs replacement Child SAs, and only then retires the old SAs.
- Keep reauth control distinct from the existing lifetime/rekey control.

## Data Flow (MANDATORY)

### Entry Point
- Config: new leaves in the ike-group container (e.g. `reauth true|false`, `reauth-time <sec>`), with an optional per-peer override (`yes`/`no`/`inherit`), in `ze-ipsec-conf.yang`.
- Resolved through the IKE config path into `IKEGroup` (`internal/component/ike/ipsec/types.go`).

### Transformation Path
1. YANG `reauth`/`reauth-time` parsed into new `IKEGroup` fields (and per-peer override).
2. When the SA reaches Established, the maintenance loop (established.go) arms a reauth timer alongside the existing lifetime/rekey timer.
3. On reauth-time expiry, run a fresh IKE_SA_INIT + IKE_AUTH while the old IKE SA and Child SAs remain valid and carry traffic.
4. After successful authentication and Child SA installation, switch forwarding to the replacement and delete the old Child SAs and IKE SA. Never remove the old path to start the handshake.
5. If replacement authentication or installation fails, remove the incomplete replacement and retain the old SAs only while their existing lifetime and authentication policy permit. Reauthentication never extends an old SA past its validity; expiry still tears it down.
6. If reauth is disabled, retain the existing rekey behavior.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ Engine | YANG reauth leaves → `IKEGroup` fields | [ ] |
| Timer ↔ SA lifecycle | reauth timer starts replacement authentication while the old SA remains owned | [ ] |
| Engine ↔ Dataplane | new Child SAs installed before forwarding switches and old Child SAs are removed | [ ] |

### Integration Points
- `established.go` maintenance loop - arm/evaluate the reauth timer next to the lifetime timer.
- `runInitiator` (`fsm.go`) - reused for re-establishment.
- `IKEGroup` (`types.go`) - new reauth fields.
- `ze-ipsec-conf.yang` - new reauth fields; preserve the current `lifetime` description.

### Architectural Verification
- [ ] No bypassed layers (config → `IKEGroup`, not a side channel)
- [ ] No unintended coupling (reauth lives in the engine, reusing `runInitiator`)
- [ ] No duplicated functionality (re-establishment reuses the initiator handshake, not a copy)
- [ ] Zero-copy preserved where applicable
- [ ] Registration over hardcoding — reauth is a config-driven engine behaviour; no new per-feature switch added to a shared struct beyond `IKEGroup` fields.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `runInitiator` can be re-invoked mid-session for a live re-establishment | reconcile loop already re-runs it on failure (reconcile.go) | reauth needs new handshake plumbing | trace `runInitiator` re-entry during audit | unvalidated |
| A-2 | A replacement can authenticate and install Child SAs while the old SAs carry traffic | make-before-break requirement in R-1 and the critical review checklist | design cannot be marked ready until coexistence and a traffic-gap bound are specified | interop test measuring successful and failed replacement transitions | unvalidated |
| A-3 | Per-peer override semantics (yes/no/inherit) match operator expectation | common ipsec model | wrong precedence surprises operators | design confirmation with user | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reauth interrupts traffic | interop test shows dropped packets | install the authenticated replacement before deleting old SAs; no handshake-duration outage is permitted. Design must state the maximum forwarding-switch interruption and its measurement before implementation |
| R-2 | Reauth storms if reauth-time is very small | many re-establishments/min | enforce a sane minimum reauth-time in YANG |
| R-3 | Older documentation conflated lifetime and reauth | operators relied on the old wording | retain the corrected rekey description; document reauth as a new opt-in control |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| ike-group `reauth true reauth-time 60` | → | maintenance loop arms reauth timer | `test/plugin/ike-reauth.ci` |
| reauth-time expires | → | fresh authentication and Child SA install before old-SA removal | `test/plugin/ike-reauth.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `reauth true`, `reauth-time 60` | at 60s a fresh IKE_SA_INIT + IKE_AUTH starts; successful replacement authenticates and installs Child SAs before old-SA deletion |
| AC-2 | reauth disabled | soft expiry still performs a rekey (unchanged) |
| AC-3 | per-peer `reauth no` over group `reauth yes` | that peer does not reauth |
| AC-4 | per-peer `reauth inherit` | peer follows the group setting |
| AC-5 | reauth-time below minimum | config verify rejects |
| AC-6 | `lifetime` leaf | retains its current rekey description and semantics |
| AC-7 | continuous traffic during successful reauth | the old path carries traffic during authentication; packet evidence records the forwarding switch and satisfies the interruption bound fixed during design. No teardown-first handshake outage is allowed |
| AC-8 | replacement authentication or installation fails | incomplete replacement state is removed; valid old SAs keep carrying traffic, and their original expiry/security policy still applies |

The numeric traffic-gap bound and failure retry policy remain design decisions.
The spec cannot become `ready` until those are recorded and the interop test has
a concrete bound to assert. Choosing teardown-first would relax the existing
make-before-break requirement and needs an owner decision.

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables reauth on a site-to-site tunnel | config → timer → fresh authentication → new Child SA install → forwarding switch → old-SA removal | `test/plugin/ike-reauth.ci` + interop |
| 2 | overrides reauth off for one peer | per-peer override resolved, peer keeps rekey | `test/plugin/ike-reauth.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReauthTimerFiresReestablish` | `internal/component/ike/engine/reauth_test.go` | reauth triggers full re-establish, not rekey | |
| `TestReauthDisabledStillRekeys` | `internal/component/ike/engine/reauth_test.go` | default path unchanged | |
| `TestReauthPeerOverride` | `internal/component/ike/engine/reauth_test.go` | yes/no/inherit precedence | |
| `TestReauthConfigParse` | `internal/component/ike/ipsec/..._test.go` | YANG reauth leaves → IKEGroup | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| reauth-time | design min..86400 | 86400 | below min | 86401 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ike-reauth` | `test/plugin/ike-reauth.ci` | reauth re-establishes; override disables per peer | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-ike-reauth-peer` | `test/interop/scenarios/` | strongSwan | fresh authentication and Child SAs precede old-SA deletion; continuous traffic satisfies AC-7, and failed replacement preserves valid old SAs under AC-8 | |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/ike/engine/established.go` - arm/evaluate reauth timer
- `internal/component/ike/engine/rekey.go` - keep rekey; ensure reauth path is distinct
- `internal/component/ike/ipsec/types.go` - add reauth fields to `IKEGroup`
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - add reauth leaves; preserve corrected `lifetime` description
- `internal/component/ike/engine/fsm.go` / `reconcile.go` - reuse `runInitiator` for scheduled re-establishment

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `ze-ipsec-conf.yang` reauth leaves; `ai/rules/config.md`, `ai/rules/config.md` |
| YANG validation constraints | [ ] yes | `reauth-time` `range`; per-peer override enum |
| CLI grammar | [ ] yes | `ai/rules/cli.md` |
| Functional test for new behaviour | [ ] yes | `test/plugin/ike-reauth.ci` |
| Prometheus counters/metrics | [ ] maybe | counter for reauth events per peer |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |
| 9 | RFC behavior implemented? | [ ] yes | RFC 7296 §2.8.3 short summary |
| 11 | Affects daemon comparison? | [ ] yes | `docs/comparison.md` |

## Files to Create
- `internal/component/ike/engine/reauth.go` - reauth timer + re-establishment orchestration
- `internal/component/ike/engine/reauth_test.go` - unit tests
- `test/plugin/ike-reauth.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)**: add reauth YANG leaves and `IKEGroup` fields; preserve lifetime semantics; add a failing `test/plugin/ike-reauth.ci`.
2. **Phase: Re-establishment path**: start fresh authentication while the old SAs remain live, install replacement Child SAs, switch forwarding, then retire the old SAs. On failure remove only the incomplete replacement and respect the old SA's original validity.
   - Tests: `TestReauthTimerFiresReestablish`, plus AC-7 transition/traffic evidence and AC-8 failure evidence.
3. **Phase: Override resolution** — group + per-peer yes/no/inherit.
   - Tests: `TestReauthPeerOverride`
4. **Phase: Validation** — reauth-time minimum.
5. **Functional + interop tests (strongSwan)**
6. **Full verification** → `./le verify current mode full`
7. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | reauth is a fresh IKE_SA_INIT+IKE_AUTH, not a CREATE_CHILD_SA rekey |
| Data flow | child SA migration installs before old removal (bounded gap) |
| Naming | YANG uses kebab-case; description no longer conflates rekey/reauth |
| Registration over hardcoding | reauth fields on `IKEGroup`, engine-driven |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| reauth path | `go test ./internal/component/ike/engine -run Reauth` |
| interop | `NN-ike-reauth-peer` against strongSwan |
| lifetime compatibility | existing rekey behavior and description remain unchanged when reauth is absent |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | reauth-time bounded; no zero/one-second storms |
| Key hygiene | old SA keys cleared after migration (as `rekeyIKESA` does via `SKKeys.Clear`) |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## RFC Documentation
Add `// RFC 7296 Section 2.8.3` comments above the reauth trigger and the
make-before-break reauthentication orchestration, distinguishing it from the §1.3.3 rekey.

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
- [ ] AC-1..AC-8 all demonstrated, including the design's numeric traffic-gap bound and failed replacement
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
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
