# Spec: eap-tls-certificate-revocation

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | spec-ipsec-rfc9190 |
| Phase | - |
| Updated | 2026-09-19 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This spec captured the missing RFC 9190 Section 5.4 revocation checks and the
owner's 2026-08-12 deferral. The current implementation is owned by
`plan/spec-ipsec-rfc9190.md`: its phases 4 and 6 record the five MUST-level
requirements as implemented on 2026-09-05 and 2026-09-08. This file remains open
for reconciliation with that owner's evidence and closure; it does not schedule
a second implementation.

### Historical owner ruling

On 2026-08-12, Thomas answered the question about absent revocation information:

> Certificate revocation: nice to have write a spec for later.

The file was then described as belonging in `plan/future/` by that ruling,
although the five MUST-level requirements remained unmet and were never
reclassified. That directory description is historical. The current
`plan/immediate/` location does not itself revoke the ruling or establish a new
release decision. The later implementation record means release planning must
assess the remaining proof, rather than schedule the original missing checks.

### Current ownership and implementation

`docs/guide/ipsec.md` and `docs/architecture/ike/ipsec-11-interop-eap.md` describe
the current behavior. Each original requirement remains assigned to the RFC 9190
spec:

| Requirement | Producer and implemented behavior |
|-------------|-----------------------------------|
| `RFC9190-5.4-1` | `checkChainRevocation` and `crlSet.checkChain` in `internal/core/eap/revocation.go` check every certificate except the trust anchor on both roles. `newTLSMethod` and `serverChainCheck.verifyConnection` install the callbacks. TLS 1.3 refuses when no CRL source is configured; TLS 1.2 can proceed without one |
| `RFC9190-5.4-2` | `newTLSMethod` in `internal/core/eap/eap_tls.go` supplies the operator's OCSP response as `tls.Certificate.OCSPStaple` |
| `RFC9190-5.4-3` | `serverChainCheck.verifyConnection` in `internal/core/eap/peer_chain.go` calls `checkStapledChainStatus` when `certificate-status-request` is enabled. Missing or invalid status refuses the handshake; intermediate CertificateEntry status that Go cannot expose also refuses |
| `RFC9190-5.4-4`, `RFC9190-5.4-5` | `startServerCertRecheck` in `internal/component/ike/engine/postauth_revocation.go` starts the post-authentication check after connectivity exists. It checks over HTTPS and reports a revocation verdict to the SA owner |

The earlier parser and staple-source proposals are covered by the implemented
`CheckCertificateStatus` in `internal/core/eap/ocsp.go` and the operator-supplied
staple. They do not authorize a second OCSP parser or a fetch-and-refresh service.
The original post-connectivity check is part of the implemented scope, not an
optional removal from this spec.

The old fail-open/fail-closed menu is no longer an undecided description of the
handshake. The current code refuses TLS 1.3 without revocation material and
refuses a missing staple when status checking is enabled. The later online check
has a separate result: an unreachable responder leaves the SA up with a warning
(`serverCertRecheck.run`). RFC 9190 Section 5.4's SHOULD NOTs about trusting the
network remain separate obligations; the RFC 9190 owner must retain them in its
evidence assessment.

### Evidence and release boundary

Implementation is not an enrolment or release verdict. `rfc/not-enrolled.txt`
still lists RFC 9190 as backlog, and `plan/spec-ipsec-rfc9190.md` owns the
remaining proof and the publication barrier. Its dated evidence and current
producers replace this file's obsolete claim that neither role checks
revocation. Reconcile this retained capture with that spec before closure,
without changing buckets or declaring a current test pass from a source read.

No `{gap}` or `{not-applicable}` annotation may be added to bypass proof of
these five requirements. The historical deferral never authorized that, and
the recorded RFC 9190 goal remains implementation followed by full proof.

---

## Implementation Summary

### What Was Implemented
- Nothing in this spec. It closes as superseded: every requirement it captured
  is implemented and owned by `plan/spec-ipsec-rfc9190.md` (phases 4 and 6).
  Closure (2026-10-08, independent `/ze-close`) verified that ownership against
  the producers and repointed the two citers.

### Bugs Found/Fixed
- None.

### Documentation Updates
- `features/ipsec-eap-authentication.md` "Defect review" row: the open-spec list
  now names `spec-ipsec-rfc9190` in place of this stem.
- `plan/spec-ipsec-rfc9190.md` Task: the pointer to this file is restated with
  the bare stem, and says that spec owns 5.4-1 to 5.4-5 and the SHOULD NOTs
  5.4-6 and 5.4-7.

### Deviations from Plan
- None: the Task already said this file schedules no second implementation.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| none | | | | |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RFC9190-5.4-1 | Done (owned by spec-ipsec-rfc9190) | `internal/core/eap/revocation.go` `checkChainRevocation`, `crlSet.checkChain` | phase 4 |
| RFC9190-5.4-2 | Done (owned by spec-ipsec-rfc9190) | `internal/core/eap/eap_tls.go` `newTLSMethod` | phase 6 |
| RFC9190-5.4-3 | Done (owned by spec-ipsec-rfc9190) | `internal/core/eap/peer_chain.go` `serverChainCheck.verifyConnection`, `internal/core/eap/ocsp.go` `checkStapledChainStatus`, `CheckCertificateStatus` | phase 6 |
| RFC9190-5.4-4, 5.4-5 | Done (owned by spec-ipsec-rfc9190) | `internal/component/ike/engine/postauth_revocation.go` `startServerCertRecheck` | phase 6 |
| RFC9190-5.4-6, 5.4-7 (SHOULD NOT) | Owned by spec-ipsec-rfc9190 | `plan/spec-ipsec-rfc9190.md` step 4 names them as not implemented | its enrolment goal covers them |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| (none) | n/a | the spec carries no AC table | its Task owns no implementation; spec-ipsec-rfc9190 AC-5 and AC-9 own the proof |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| (none) | n/a | - | the tests live in spec-ipsec-rfc9190's TDD plan |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| (none) | n/a | no file was planned here |

### Audit Summary
- **Total items:** 6 requirement rows
- **Done:** 5 MUST rows, by spec-ipsec-rfc9190
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0 (the SHOULD NOT row stays with its owner)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Reconcile this capture with spec-ipsec-rfc9190's evidence, without a second implementation | ownership check + unit tests + interop | rfc9190 spec "What is missing" row "OCSP stapling and revocation: IMPLEMENTED" and step 4; 2026-10-08 run of the RFC9190 revocation, OCSP and post-auth tests in `internal/core/eap` and `internal/component/ike/engine`: all PASS; interop scenario `test/interop-ipsec/scenarios/responder-eap-tls13-revoked-client` exists with its red phase recorded in that spec |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| RFC 9190 enrolment and remaining proof (AC-9 audit verdicts, SHOULD NOTs 5.4-6/5.4-7) | never this spec's work | `plan/spec-ipsec-rfc9190.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/eap-tls-certificate-revocation-<session>.md`, recorded by `./le spec review record` at closure |
| `./le spec review check` | clean |
| Rounds | 1 |
| Reviewer lenses used | ownership (every requirement mapped to a producer and to the owning spec), citer completeness (`git grep` over the tracked tree), no-annotation rule |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| - | none | 0 BLOCKER, 0 ISSUE | - | - |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/eap/revocation.go`, `internal/core/eap/ocsp.go`, `internal/component/ike/engine/postauth_revocation.go` | yes | `git grep '^func'` finds `checkChainRevocation`, `checkStapledChainStatus`, `CheckCertificateStatus`, `startServerCertRecheck` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| 5.4-1..5.4-5 | implemented and tested by the owner spec | 2026-10-08 `go test -run 'PostAuthenticationCheck\|EAPTLS13RefusesARevoked\|EAPTLS13Staples\|EAPTLS13PeerRefuses'`: 16 PASS, `ok internal/core/eap`, `ok internal/component/ike/engine` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| EAP-TLS 1.3 server against strongSwan with a revoked client | `test/interop-ipsec/scenarios/responder-eap-tls13-revoked-client` | exists (ls); red phase recorded 2026-09-05 in spec-ipsec-rfc9190; not re-run here (no Docker) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| (none declared) | n/a | the spec declares no A-N rows |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `features/ipsec-eap-authentication.md` describes stapling, stapled-status refusal and the post-auth recheck | producers above | yes; only the Defect review row changed |
