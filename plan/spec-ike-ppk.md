# Spec: ike-ppk

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-04 |

Anchor refresh (2026-07-22 plan review, design still valid, feature not
landed): the key-derive call site cited as `fsm.go` moved ~150 lines
to `fsm.go` (`DeriveSKKeys` `:464`, `sa.SKKeys=` `:475`).
`payload_notify.go` and `keys.go/45` still hold. Re-read by symbol name,
not cited line; also re-verify the `engine/rekey.go,189` rekey site at
implementation start.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/ike/wire/payload_notify.go` - IKEv2 notify-type registry
4. `internal/component/ike/crypto/keys.go` - SK_* key derivation
5. `internal/component/ike/engine/fsm.go` - where keys are derived during IKE_AUTH

## Task

Ze's native IKEv2 has no Post-quantum Preshared Key (PPK) support (RFC 8784,
"Mixing Preshared Keys in IKEv2 for Post-quantum Security"). PPK mixes a static
out-of-band shared secret, selected by a PPK_ID, into the SK_d / SK_pi / SK_pr
key material so a future quantum adversary who breaks the (classical) DH exchange
still cannot derive session keys. It is complementary to, and independent of,
post-quantum key exchange (that separate track is `spec-ike-post-quantum.md`).

Add RFC 8784 PPK:
- Negotiate PPK use via the USE_PPK notification in IKE_SA_INIT.
- Carry the selected key via a PPK_IDENTITY notification in IKE_AUTH.
- Mix the PPK into SK_d/SK_pi/SK_pr after the standard derivation.
- Support the mandatory/optional policy (NO_PPK_AUTH fallback vs fail-closed).
- Configure a keyed PPK list (id → secret, plus a `mandatory` flag).

## Required Reading

### Architecture Docs
- [ ] `internal/component/ike/` - IKE component structure and engine flow.
  → Constraint: Ze's IKEv2 is a native implementation; PPK must be built on the wire/crypto/engine layers, not delegated.
- [ ] `ai/rules/config.md`, `ai/rules/config.md` - the new PPK config.
  → Constraint: the PPK secret is a `$9$`-encoded secret leaf like the existing PSK.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7296.md` - IKEv2: notify payloads (Section 3.10), SK_* derivation (Section 2.14).
- [ ] RFC 8784 - Mixing Preshared Keys in IKEv2 for Post-quantum Security (run `/ze-rfc` to add `rfc/short/rfc8784.md` before implementation).

**Key insights:**
- USE_PPK = 16435, PPK_IDENTITY = 16436, NO_PPK_AUTH = 16437 (IKEv2 status-type notifies).
- The mix step recomputes `SK_d = prf+(PPK, SK_d)`, `SK_pi = prf+(PPK, SK_pi)`, `SK_pr = prf+(PPK, SK_pr)` only when a PPK is in use; other SK_* are unchanged.
- If the peer omits USE_PPK: a `mandatory` PPK fails the exchange; an optional PPK proceeds without the mix (using NO_PPK_AUTH semantics).

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/wire/payload_notify.go` - the notify-type constant block ends at `NotifySignatureHashAlgorithms uint16 = 16431` (payload_notify.go). None of USE_PPK/PPK_IDENTITY/NO_PPK_AUTH (16435/16436/16437) exist.
- [ ] `internal/component/ike/crypto/keys.go` - `DeriveSKKeys` (keys.go) expands SKEYSEED into SK_d..SK_pr via `PRFPlus` with no PPK argument and no post-derivation mix; `DeriveSKEYSEED` (keys.go) is plain RFC 7296.
- [ ] `internal/component/ike/engine/fsm.go` - `DeriveSKEYSEED`/`DeriveSKKeys` are called and the result stored to `sa.SKKeys` during IKE_AUTH (fsm.go, `DeriveSKKeys` :464, `sa.SKKeys=` :475); the rekey path mirrors it at `engine/rekey.go` (cited :180,:189 -- re-verify by symbol, the file grew ~200 lines).

**Behavior to preserve:**
- Non-PPK sessions negotiate and derive keys exactly as today.
- All existing notify types, PSK/x509/EAP auth modes, and the wire format round-trip unchanged.
- The mix must not alter SK_ai/SK_ar/SK_ei/SK_er (data-channel keys) per RFC 8784.

**Behavior to change:**
- IKE_SA_INIT sends/parses USE_PPK; IKE_AUTH sends/parses PPK_IDENTITY.
- Key derivation gains an optional PPK mix step for SK_d/SK_pi/SK_pr.
- New PPK config (id → secret, mandatory flag) drives selection and policy.

## Data Flow (MANDATORY)

### Entry Point
- Wire: USE_PPK notify in the received/sent IKE_SA_INIT; PPK_IDENTITY notify in IKE_AUTH.
- Config: a new PPK list (each entry: `id`, `$9$`-encoded `secret`) plus a per-connection `mandatory` flag under the IKE authentication config.

### Transformation Path
1. Initiator includes USE_PPK in IKE_SA_INIT when a PPK is configured; responder that supports PPK echoes willingness.
2. During IKE_AUTH the initiator selects a configured PPK_ID and sends PPK_IDENTITY; the responder looks the ID up in its PPK list.
3. After the standard `DeriveSKKeys`, if a PPK is agreed, recompute SK_d/SK_pi/SK_pr as `prf+(PPK, SK_x)` before the AUTH payload is verified.
4. Policy resolution: if no USE_PPK is seen and the local PPK is `mandatory`, fail; if optional, continue without the mix (NO_PPK_AUTH path).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Wire ↔ engine | new notify types parsed/emitted in SA_INIT and AUTH | [ ] |
| Config ↔ engine | PPK list (id→secret, mandatory) resolved by PPK_ID | [ ] |
| Engine ↔ crypto | PPK mix recomputes SK_d/SK_pi/SK_pr | [ ] |

### Integration Points
- `internal/component/ike/wire/payload_notify.go` - add USE_PPK/PPK_IDENTITY/NO_PPK_AUTH constants (after :39) and any parse/marshal helpers.
- `internal/component/ike/engine/initiator.go` / responder - build/parse the PPK notifies in SA_INIT and AUTH.
- `internal/component/ike/crypto/keys.go` - a PPK-mix helper; invoked at the `engine/fsm.go` call site and the rekey path.
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - PPK config under `authentication` (:330); threaded through `ipsec/types.go` and `ipsec/config.go`.

### Architectural Verification
- [ ] No bypassed layers (notifies go through the wire layer; keys through crypto)
- [ ] No unintended coupling (PPK list resolution stays in the ipsec config/engine, not core)
- [ ] No duplicated functionality (single PPK-mix helper used by initial derive + rekey)
- [ ] Registration over hardcoding - new notify types extend the existing notify registry; no per-feature branch added to a core/shared package outside the IKE component.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | PPK mix only touches SK_d/SK_pi/SK_pr | RFC 8784 Section 3 | wrong keys mixed → auth/interop failure | interop test vs strongSwan PPK | unvalidated |
| A-2 | `DeriveSKKeys` is the sole SK_* producer for initial + rekey | keys.go, fsm.go, rekey.go (rekey cite unverified after file growth; re-check by symbol) | a mix site is missed | audit both call sites | unvalidated |
| A-3 | USE_PPK belongs in IKE_SA_INIT, PPK_IDENTITY in IKE_AUTH | RFC 8784 Section 3 | negotiation fails | interop test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Mixing at the wrong exchange step corrupts AUTH verification | AUTH fails on both peers | mix after `DeriveSKKeys`, before AUTH verify; unit-test key vectors |
| R-2 | Fail-open when `mandatory` PPK is configured but peer lacks it | session establishes without PPK | explicit policy gate; test the mandatory-reject path |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PPK configured + USE_PPK exchanged | → | SK_d/SK_pi/SK_pr mixed with PPK | `test/ci/ike-ppk.ci` |
| `mandatory` PPK, peer omits USE_PPK | → | exchange fails closed | `test/ci/ike-ppk.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | both peers configured with matching PPK id/secret | SA establishes; SK_d/SK_pi/SK_pr mixed |
| AC-2 | PPK optional, peer lacks PPK | SA establishes without mix (NO_PPK_AUTH) |
| AC-3 | PPK mandatory, peer lacks PPK | SA fails with a clear reason |
| AC-4 | mismatched PPK secret, same id | AUTH fails on both peers |
| AC-5 | no PPK configured | behaviour identical to today |
| AC-6 | RFC 8784 key vector | mixed SK_d/SK_pi/SK_pr match expected bytes |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | configures a quantum-resistant PPK on both ends of a tunnel | config PPK list → USE_PPK/PPK_IDENTITY → PPK mix | `test/ci/ike-ppk.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPPKNotifyRoundTrip` | `internal/component/ike/wire/payload_notify_test.go` | USE_PPK/PPK_IDENTITY/NO_PPK_AUTH marshal+parse | |
| `TestDeriveSKKeysPPKMix` | `internal/component/ike/crypto/keys_test.go` | SK_d/SK_pi/SK_pr recomputed; others unchanged (RFC 8784 vector) | |
| `TestPPKPolicyMandatory` | `internal/component/ike/engine/..._test.go` | mandatory-without-peer fails; optional proceeds | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| PPK_ID length | 1..255 bytes | 255 | 0 (empty id) | 256 |
| PPK secret | non-empty | - | empty rejected | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ike-ppk` | `test/ci/ike-ppk.ci` | tunnel with matching PPK establishes; mandatory-mismatch fails | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| PPK tunnel Ze ↔ peer | `test/ci/` | strongSwan (PPK) | RFC 8784 interop, correct key mixing | |

### Future (if deferring any tests)
- None planned.

## RFC Documentation
- RFC 7296 (`rfc/short/rfc7296.md`): notify payload framing (Section 3.10), SK_* derivation (Section 2.14) - the layers PPK extends.
- RFC 8784: PPK negotiation and the SK_d/SK_pi/SK_pr mix; add `rfc/short/rfc8784.md` via `/ze-rfc` before implementation.

## Files to Modify
- `internal/component/ike/wire/payload_notify.go` - USE_PPK/PPK_IDENTITY/NO_PPK_AUTH notify types + helpers
- `internal/component/ike/engine/initiator.go` - build/parse PPK notifies in SA_INIT and AUTH
- `internal/component/ike/crypto/keys.go` - PPK-mix helper for SK_d/SK_pi/SK_pr
- `internal/component/ike/engine/fsm.go` - invoke the mix at the derive call site (and `rekey.go`)
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - PPK list + mandatory flag under `authentication`
- `internal/component/ike/ipsec/config.go`, `internal/component/ike/ipsec/types.go` - parse/carry PPK config

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | [ ] yes | `ze-ipsec-conf.yang` PPK list; `ai/rules/config.md` |
| Functional test | [ ] yes | `test/ci/ike-ppk.ci` |
| Interop test | [ ] yes | strongSwan PPK peer |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `test/ci/ike-ppk.ci` - functional + interop test
- `rfc/short/rfc8784.md` - RFC summary (via `/ze-rfc`)
- (unit tests extend existing test files)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. RFC summary | `/ze-rfc` for RFC 8784 |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the three notify constants + PPK config (parsed, unused); failing `test/ci/ike-ppk.ci`.
2. **Phase: Notify negotiation** - send/parse USE_PPK (SA_INIT) and PPK_IDENTITY (AUTH).
   - Tests: `TestPPKNotifyRoundTrip`
3. **Phase: Key mix** - PPK-mix helper for SK_d/SK_pi/SK_pr at derive + rekey sites.
   - Tests: `TestDeriveSKKeysPPKMix`
4. **Phase: Policy** - mandatory/optional resolution incl. NO_PPK_AUTH fallback.
   - Tests: `TestPPKPolicyMandatory`
5. **Functional + interop test (strongSwan)**
6. **Full verification** → `make ze-precommit-verify`
7. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | only SK_d/SK_pi/SK_pr mixed; matches RFC 8784 vector; mandatory fails closed |
| Wire correctness | notify types parse/marshal at correct exchange steps |
| Registration over hardcoding | notify types extend the existing registry; PPK logic stays in the IKE component |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| notify types | `go test ./internal/component/ike/wire -run PPK` |
| key mix | `go test ./internal/component/ike/crypto -run PPK` |
| interop | `test/ci/ike-ppk.ci` vs strongSwan |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Fail-closed | mandatory PPK never establishes without the mix |
| Secret handling | PPK secret zeroed after use like other key material |
| Input validation | PPK_ID length bounds; empty secret rejected |

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (PPK_ID length)
- [ ] Functional + interop tests for end-to-end behavior
