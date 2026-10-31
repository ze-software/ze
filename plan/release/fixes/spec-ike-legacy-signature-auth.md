# Spec: ike-legacy-signature-auth

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | A certificate-authenticated Ze cannot authenticate to an IKEv2 peer that lacks RFC 7427: `computeX509Auth` refuses to sign when no SIGNATURE_HASH_ALGORITHMS notify arrived, and RFC 8247 Section 3.1 makes RSA Digital Signature (method 1) a MUST. Method 1 is also verified with SHA-256 only, so a peer signing with the RFC 7296 default hash (SHA-1) is refused |
| RFC gaps | RFC8247-3.1-1, RFC4754-7-1 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-31 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Approval

Thomas approved this design on 2026-10-31, covering both RSA and ECDSA: with no
SIGNATURE_HASH_ALGORITHMS notify received, Ze signs with the legacy method that
fits its key (method 1 with RSASSA-PKCS1-v1_5 and SHA-1 for RSA; methods 9, 10
and 11 by curve for ECDSA), refuses any other key with an error naming the cause
and the fix, and keeps method 14 unchanged when the notify arrived.

## Ownership

Worked by a subagent of session bc566aa5-3f69-415d-8c8b-c6f8cdf8adae, without a
`./le spec claim` marker: the claim was refused because 13 specs are already
in-progress against a cap of 12, and the cap is lowered as specs close, never
raised. The status stays `ready` for that reason, not because work has not
started.

## Task

`computeX509Auth` (`internal/component/ike/engine/auth.go`) only signs with
method 14 (RFC 7427 Digital Signature) and returns `errNoSignatureHashAlgos` when
the peer sent no SIGNATURE_HASH_ALGORITHMS notify. A peer without RFC 7427
support (strongSwan with `charon.signature_authentication = no`, older
implementations) can therefore never be authenticated to by a Ze that holds a
certificate. RFC 8247 Section 3.1 lists RSA Digital Signature as MUST, which
`rfc/short/rfc8247.md` records as the `{gap}` on RFC8247-3.1-1.

The goal: with no notify received, Ze signs with method 1 for an RSA key and
with method 9, 10 or 11 for an ECDSA key on P-256, P-384 or P-521; any other key
is refused with a plain error. The method 1 verifier accepts the SHA-1 default
the RFC names, so Ze verifies what strongSwan sends under the same condition.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/ike/ipsec-7-ikev2-engine.md` - "A signature AUTH is verified by the method number the peer sent"
  → Constraint: the paragraph says method 14 is the only signature method Ze signs with and that method 1 is verified with SHA-256; both sentences change with this work
- [ ] `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` - the RFC 7427 conformance coverage that `rfc7427_method_test.go` declares as its design page
  → Constraint: reread at closure for any claim that method 14 is the only signature Ze sends
- [ ] `ai/doors/access-security.md` - IKE engine boundaries
  → Constraint: tests of `internal/component/ike/engine` link `internal/core/eap`, so they run through `./le`, which selects the patched GOROOT

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8247.md` - RFC8247-3.1-1 MUST, recorded `{gap}`
  → Decision: the gap note names both halves (no method 1 sent, SHA-256-only verifier); both are closed here
- [ ] `rfc/short/rfc4754.md` - RFC4754-7-1 SHALL, verify side only
  → Decision: the signing side is added; the enrolment reason and Ze context paragraphs change from "never signs"
- [ ] `rfc/short/rfc7427.md` - RFC7427-3-4 and RFC7427-3-5
  → Constraint: method 14 stays exactly as it is when the notify arrived; the two tests that assert "no payload without the notify" change and need the `ze-test-judge`
- [ ] `rfc/full/rfc7296.txt` Section 3.8, method 1: "To promote interoperability, implementations that support this type SHOULD support signatures that use SHA-1 as the hash function and SHOULD use SHA-1 as the default hash function when generating signatures."
  → Decision: sign method 1 with SHA-1; verify SHA-1 and the SHA-2 hashes a DigestInfo may name
- [ ] `rfc/full/rfc4754.txt` Section 7: "The bit lengths of r and s are enforced, if necessary, by pre-pending the value with zeros."
  → Decision: the signer zero-pads r and s to 32, 48 or 66 octets, the inverse of `verifyECDSAAuth`
- [ ] `rfc/full/rfc7427.txt` Section 3: "the peer is only allowed to use this authentication method if the Notify payload of type SIGNATURE_HASH_ALGORITHMS has been sent and received by each peer."
  → Constraint: no notify received means method 14 is not permitted, so the legacy method is the only signature available

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/ike/engine/auth.go` (1004L) - `computeX509Auth` refuses with `errNoSignatureHashAlgos` on an empty `sa.RemoteHashAlgos`; `selectSignatureAlgorithm` and `signDigest` build method 14; `verifyLegacyRSAAuth` verifies method 1 with SHA-256 only
- [ ] `internal/component/ike/engine/auth_ecdsa.go` (106L) - `ecdsaMethod` table and `verifyECDSAAuth`, r || s fixed-length decode
- [ ] `internal/component/ike/engine/rfc7427_method_test.go` - `TestRFC7427DigitalSignatureMethodIsUsed`, negative half asserts no payload without the notify
- [ ] `internal/component/ike/engine/rfc7427_sighash_test.go` - `TestRFC7427DigitalSignatureNeedsTheNotify`, negative half asserts `errNoSignatureHashAlgos`
- [ ] strongSwan 6.1.0 `src/libcharon/sa/ikev2/authenticators/pubkey_authenticator.c` - `build` signs AUTH_RSA with SIGN_RSA_EMSA_PKCS1_SHA1 and ECDSA by key size (256, 384, 521); `process` verifies AUTH_RSA with SHA-1 only
- [ ] strongSwan 6.1.0 `src/libcharon/sa/ikev2/tasks/ike_init.c` - `charon.signature_authentication` (default yes) gates both sending and processing the notify

**Behavior to preserve:**
- Method 14 selection, encoding and the RFC 7427 Section 4 algorithm choice when the notify arrived
- Every verify path for methods 9, 10, 11 and 14

**Behavior to change:**
- No notify received: sign with method 1 (RSA, SHA-1) or 9/10/11 (ECDSA by curve) instead of refusing
- Unsupported key with no notify: refusal naming the key type, the cause and the fix
- `GODEBUG=fips140=only`: the method 1 signer refuses with a plain error, because Go refuses SHA-1 signatures in that mode
- Method 1 verify accepts SHA-1 (and SHA-2-256/384/512 DigestInfo)

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- IKE_AUTH construction for a peer configured `authentication { mode x509; }`, initiator (`initiator.go`) or responder (`responder.go`), through `computeLocalAuth`

### Transformation Path
1. IKE_SA_INIT processing stores the peer's SIGNATURE_HASH_ALGORITHMS list in `sa.RemoteHashAlgos` (empty when none arrived)
2. `computeLocalAuth` dispatches `AuthX509` to `computeX509Auth`
3. `computeX509Auth` loads the certificate and key, computes the signed octets, and branches on the received list: method 14 when present, the legacy signer when absent
4. The AUTH payload is encoded by `wire.PayloadAUTH` unchanged

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ IKE peer | AUTH payload in IKE_AUTH | Yes, strongSwan interop |

### Integration Points
- `pki.GetCertificate` supplies the private key; no new registration

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the branch sits inside `computeX509Auth`, the single X.509 signer |
| No unintended coupling (components stay isolated) | Yes | engine-internal change only |
| No duplicated functionality (extends existing, does not recreate) | Yes | the ECDSA signer reuses the `ecdsaMethod` table the verifier reads |
| Zero-copy preserved where applicable (refs, not copies) | N-A | control plane, once per IKE_AUTH |
| Registration over hardcoding, outbound | N-A | no new command, family or handler |
| Registration over hardcoding, inbound | Yes | the method numbers live on the `ecdsaMethod` values; no switch outside the signer learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | strongSwan with `charon.signature_authentication = no` sends no notify and signs with method 1 (SHA-1) or 9/10/11 | `ike_init.c` line 441 and `pubkey_authenticator.c` build | the interop scenario would not exercise the fallback | interop checker reads charon's "with RSA signature successful" / "ECDSA-2xx signature" lines; the computeX509Auth revert went red in all four | confirmed |
| A-2 | Ze is never built or run in Go FIPS 140-3 mode | no `GOFIPS140`, `fips140` or `//go:debug` in build, appliance or launcher code; `spec-crypto-policy.md` says there is no FIPS mode | a SHA-1 signature would fail | `git grep` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Accepting SHA-1 in method 1 verification weakens verification | review | RFC 7296 Section 3.8 asks for SHA-1 support; RFC 8247 Section 3.1 keeps method 1 as MUST; method 14 remains preferred whenever both sides notify |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | X.509 IKE_AUTH against a peer without RFC 7427 |
| How is it reverted? | single commit revert |
| Who else touches this path? | none in flight |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `computeLocalAuth` with `AuthX509` and no notify | → | `computeX509Auth` legacy branch | `TestRFC8247LegacySignatureWithoutNotify` |
| strongSwan IKE_AUTH with RFC 7427 disabled | → | `computeX509Auth`, `verifyLegacyRSAAuth`, `verifyECDSAAuth` | interop `x509-legacy-rsa-signature`, `x509-legacy-ecdsa-p256-signature`, `x509-legacy-ecdsa-p384-signature`, `x509-legacy-ecdsa-p521-signature` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | RSA key, no notify received | AUTH method 1, RSASSA-PKCS1-v1_5 over SHA-1 of the signed octets, verified by `verifyLegacyRSAAuth` and by `rsa.VerifyPKCS1v15(…, crypto.SHA1, …)` |
| AC-2 | ECDSA P-256 / P-384 / P-521 key, no notify received | AUTH method 9 / 10 / 11, r || s of 64 / 96 / 132 octets, verified by `verifyECDSAAuth` and by `ecdsa.Verify` with SHA-256 / SHA-384 / SHA-512 |
| AC-3 | Notify received | method 14 exactly as before |
| AC-4 | Ed25519 key (or an ECDSA curve with no method), no notify received | error wrapping `errUnsupportedKey`, naming the key, the cause and the fix |
| AC-5 | `GODEBUG=fips140=only`, RSA key, no notify | error `errLegacyRSAFIPSOnly` naming fips140=only and the fix, never a silent or Go-internal failure |
| AC-6 | Peer sends method 1 signed with SHA-1 | Ze verifies it |
| AC-7 | strongSwan with RFC 7427 disabled, RSA and ECDSA (P-256, P-384, P-521) certificates | IKE and Child SA establish; charon logs Ze's AUTH verified, and its own signed, as "RSA signature" / "ECDSA-256 signature" / "ECDSA-384 signature" / "ECDSA-521 signature" (auth_method_names), Ze initiating, and for RSA also Ze responding |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC8247LegacySignatureWithoutNotify` | `internal/component/ike/engine/rfc8247_legacy_auth_test.go` | AC-1, AC-2, AC-3 | |
| `TestRFC8247LegacySignatureRefusesUnsupportedKey` | same | AC-4 | |
| `TestLegacyRSASignatureRefusedInFIPSOnlyMode` | same | AC-5 | |
| `TestRFC8247LegacyRSAVerifiesSHA1` | same | AC-6 | |
| `TestRFC4754SignerPadsRAndS` | `internal/component/ike/engine/rfc8247_legacy_auth_test.go` | AC-2 encoding, short r and short s | |
| `TestRFC7427EmptyNotifyStillRefusesLegacyMethods` | same | a received empty notify never falls back to a legacy method | |
| `TestRFC7427DigitalSignatureMethodIsUsed` (changed) | `rfc7427_method_test.go` | AC-3, no legacy method once the notify arrived | |
| `TestRFC7427DigitalSignatureNeedsTheNotify` (changed) | `rfc7427_sighash_test.go` | method 14 not used without the notify | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| r, s octets | 32/48/66 per method | short r padded to full length | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | the user path is IKE_AUTH against a real peer, which the interop scenarios exercise | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `x509-legacy-rsa-signature` | `test/interop-ipsec/scenarios/` | strongSwan 6.1.0 | method 1 both ways | |
| `x509-legacy-ecdsa-p256-signature` | same | strongSwan 6.1.0 | method 9 both ways | |
| `x509-legacy-ecdsa-p384-signature` | same | strongSwan 6.1.0 | method 10 both ways | |
| `x509-legacy-ecdsa-p521-signature` | same | strongSwan 6.1.0 | method 11 both ways | |
| `x509-legacy-rsa-signature-ze-responder` | same | strongSwan 6.1.0 | method 1 both ways, Ze responding | |

## Files to Modify
- `internal/component/ike/engine/auth.go` - `computeX509Auth` branch; `verifyLegacyRSAAuth` moves out
- `internal/component/ike/engine/auth_ecdsa.go` - method number on `ecdsaMethod`, curve lookup, signer
- `internal/component/ike/engine/rfc7427_method_test.go`, `rfc7427_sighash_test.go` - negative halves
- `internal/le/interoplab/ipsec/checkers.go` - four scenario entries
- `rfc/short/rfc8247.md`, `rfc/short/rfc4754.md`, `rfc/short/rfc7427.md` (if a claim changes)
- `docs/architecture/ike/ipsec-7-ikev2-engine.md`

## Files to Create
- `internal/component/ike/engine/auth_legacy.go` - legacy signer dispatch, method 1 signer and verifier
- `internal/component/ike/engine/rfc8247_legacy_auth_test.go`
- `internal/le/interoplab/ipsec/legacysig.go` - checkers
- `test/interop-ipsec/scenarios/x509-legacy-*-signature/` - four scenarios, three with their own `pki/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no config added: the method follows the peer's notify |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | No | - |
| CLI grammar (keyword before value) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |
| Pipe completeness | No | - |
| Env var registration | No | - |
| Doctor check for runtime dependencies | No | no new dependency |
| Prometheus counters/metrics | No | - |
| BGP family surface (new SAFI / capability / attribute) | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | Yes | `docs/architecture/ike/ipsec-7-ikev2-engine.md` (methods Ze sends) |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc8247.md`, `rfc/short/rfc4754.md` |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` if it lists IPsec scenarios |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/ipsec.md` gains the signature-method paragraph. Unaffected, read for a method claim and holding none: `docs/architecture/ike/ipsec-11-interop-eap.md`, `ipsec-13-rekey-wire.md`, `ipsec-14-responder.md`, `rfcgate-1b-rfc7296-pilot.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

1. **Phase: Wiring** -- the legacy branch in `computeX509Auth`, failing test `TestRFC8247LegacySignatureWithoutNotify`
2. **Phase: Signers and verifier** -- method 1 signer with the fips140=only refusal, ECDSA signer, method 1 verifier accepting SHA-1; the two RFC 7427 tests changed under the judge
3. **Phase: Interop** -- four strongSwan scenarios with `charon.signature_authentication = no`, revert/red/restore/green walk
4. **Phase: Records and docs** -- `./le rfc record` for the new tags and every record the producer change stales; summaries and the engine page

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | method 14 path byte-identical when the notify arrived; r and s left-padded |
| Rule: rfc-compliance | quotes above each new implementing function |
| Rule: user-facing-errors | refusals name cause and fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| legacy signer | `./le test unit` on `internal/component/ike/engine` |
| interop scenarios | `./le test integration interop-ipsec` with the scenario selector |
| RFC rows | `./le rfc check`, `./le rfc tree` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Downgrade | the legacy method is used only when no notify arrived; a received notify always yields method 14 |
| Verification breadth | method 1 verify accepts only SHA-1 and SHA-2 DigestInfo; nothing else |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior |
| 3 fix attempts failed | STOP. Report all 3 approaches |

## Design Insights

- Ze's method 1 verifier used SHA-256 only, while strongSwan verifies and signs method 1 with SHA-1 only (`pubkey_authenticator.c`), so the two never interoperated on method 1 in either direction.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Sign method 1 with SHA-1 | SHA-256 in the DigestInfo | RFC 7296 Section 3.8 names SHA-1 as the default; strongSwan verifies SHA-1 only |
| Refuse in fips140=only before signing | let `rsa.SignPKCS1v15` fail | the Go error names no IKE cause or fix |
| ECDSA method by curve | by hash preference | RFC 4754 Section 4 binds each method to one curve |

## Known Limitations
- None.

## RFC Documentation (Scope: protocol)

Each new signer carries the RFC 7296 Section 3.8, RFC 4754 Section 7 and RFC 8247
Section 3.1 quotes above the implementing statement.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `computeX509Auth` signs with the key's legacy method when `sa.RemoteHashAlgos` is nil (no notify received): `auth_legacy.go::computeLegacySignatureAuth`, `signLegacyRSAAuth` (method 1, SHA-1), `auth_ecdsa.go::signECDSAAuth` (methods 9, 10, 11 by curve, r || s padded). Other keys refuse with `errUnsupportedKey` naming the key and the fix.
- `GODEBUG=fips140=only`: method 1 signing refuses with `errLegacyRSAFIPSOnly`; method 1 verify skips SHA-1 and names the mode.
- `verifyLegacyRSAAuth` moved to `auth_legacy.go`, verifies SHA-1, SHA-256, SHA-384 and SHA-512 DigestInfo.
- `parseHashAlgoNotify` never answers nil, so a received empty notify keeps method 14 mandatory.

### Bugs Found/Fixed
- Method 1 was verified with SHA-256 only, so Ze refused every method 1 AUTH strongSwan sends (SHA-1). Fixed in `verifyLegacyRSAAuth`; the RSA interop scenario went red with the old verifier.

### Documentation Updates
- `docs/architecture/ike/ipsec-7-ikev2-engine.md`, `docs/guide/ipsec.md`, `docs/architecture/testing/interop.md`, `rfc/short/rfc8247.md`, `rfc/short/rfc4754.md`, `rfc/short/rfc7427.md`.

### Deviations from Plan
- A fifth scenario, `x509-legacy-rsa-signature-ze-responder`, added at review.
- No `./le spec claim`: the WIP cap refused it (see Ownership).

## Mistake Log
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location |
|-------------|--------|----------|
| RSA no notify: method 1 SHA-1 | done | `auth_legacy.go::signLegacyRSAAuth` |
| ECDSA no notify: 9/10/11 by curve, r||s padded | done | `auth_ecdsa.go::signECDSAAuth` |
| Other key: refusal with cause and fix | done | `auth_legacy.go::computeLegacySignatureAuth`, `signECDSAAuth` |
| Notify received: method 14 unchanged | done | `auth.go::computeX509Auth` |
| FIPS: plain refusal | done | `auth_legacy.go` |

### Acceptance Criteria
| AC | Status | Evidence |
|----|--------|----------|
| AC-1 | done | `TestRFC8247LegacySignatureWithoutNotify` |
| AC-2 | done | same, `TestRFC4754SignerPadsRAndS` |
| AC-3 | done | `TestRFC7427DigitalSignatureMethodIsUsed`, `TestRFC7427EmptyNotifyStillRefusesLegacyMethods` |
| AC-4 | done | `TestRFC8247LegacySignatureRefusesUnsupportedKey` |
| AC-5 | done | `TestLegacyRSASignatureRefusedInFIPSOnlyMode` |
| AC-6 | done | `TestRFC8247LegacyRSAVerifiesSHA1` |
| AC-7 | done | five `x509-legacy-*` interop scenarios |

### Tests from TDD Plan
| Test | Status |
|------|--------|
| every unit test in the TDD plan | pass, `go test -race` of `internal/component/ike/engine` |
| interop scenarios | pass |

### Files from Plan
| File | Status |
|------|--------|
| all Files to Modify and Files to Create | present |

### Audit Summary
- All requirements and ACs done.

## Goal Validation (BLOCKING)

| Goal | Evidence |
|------|----------|
| A certificate-authenticated Ze authenticates to a peer without RFC 7427 | interop: strongSwan 6.1.0 with `charon.signature_authentication = no`, all five `x509-legacy-*` scenarios pass; with `computeX509Auth` reverted each goes red (Ze logs `build IKE_AUTH failed`), and green after restore, 2026-10-31 |
| Ze verifies a legacy-method peer | the same scenarios bring the Child SA up after Ze verified charon's method 1 / 9 / 10 / 11 AUTH; RSA red with the SHA-256-only verifier |

## Work Not Done
- None.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/ike-legacy-signature-auth-bc566aa5-3f69-415d-8c8b-c6f8cdf8adae.md` |
| `./le spec review check` | clean |
| Rounds | 2 |
| Reviewer lenses used | ze-review (logic, wiring, RFC quotes, errors), ze-rfc-audit, ze-test-judge |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | nil-versus-empty guard untested through `computeX509Auth` | `auth.go` | `TestRFC7427EmptyNotifyStillRefusesLegacyMethods` |
| 2 | ISSUE | refusal printed a Go type name | `auth_legacy.go` | `legacyKeyName` |
| 3 | ISSUE | judge approval not found | approvals | recorded by the judge under the main thread's commit namespace `6a8796af` |

## Pre-Commit Verification

### Files Exist (ls)
- `auth_legacy.go`, `rfc8247_legacy_auth_test.go`, `legacysig.go`, five scenario directories: present.

### AC Verified (grep/test)
- engine, `interoplab/ipsec` and `test/interop-ipsec` package tests pass with `-race`.

### Wiring Verified (end-to-end)
- `computeLocalAuth` and `responder_eap.go` reach `computeX509Auth`; the interop scenarios exercise initiator and responder.

### Assumptions Resolved
- A-1 confirmed, A-2 confirmed.

### Documentation Verified
- Pages above updated in the same change.

## Core Insight
- strongSwan signs and verifies method 1 with SHA-1 only, so a SHA-256 method 1 verifier never interoperated in either direction.
