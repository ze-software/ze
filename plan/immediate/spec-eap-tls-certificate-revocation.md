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
