# Spec: eap-tls-certificate-revocation

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Updated | 2026-08-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Ze does not check whether a certificate has been revoked, in either direction.**
A certificate that an authority has cancelled continues to authenticate until it
expires on its own.

This covers five RFC 9190 Section 5.4 requirements: `RFC9190-5.4-1` through
`RFC9190-5.4-5`.

### Why this spec is in `plan/future/`

`plan/future/README.md` refuses defects, and it names "omits bytes an RFC
requires" as one. **These five are unmet MUST-level requirements. This spec is
here by an owner ruling, not because the requirements were reclassified.**

Owner ruling, 2026-08-12, in answer to a direct question about what Ze should do
when no revocation information is available: **"Certificate revocation: nice to
have write a spec for later."**

No annotation was written into `rfc/requirements/rfc9190.md`. The rows stay
unproven and honest. **RFC 9190 is not enrolled** (`rfc/not-enrolled.txt`, marked
`backlog`, 49 of 51 gated MUSTs unproven), so no ratchet fires and no gate turns
red because of this deferral. Enrolling RFC 9190 later requires these five to be
proven or to carry an owner-authorised annotation, so this spec is a
precondition of that enrolment.

### Current behavior

`verifyServerChain` (`internal/core/eap/peer.go`) calls
`certs[0].Verify(opts)`. It passes no certificate revocation list and consults no
OCSP responder. `startTLSClient` in the same file sets no `VerifyConnection`
callback, so nothing later inspects the chain either.

The server side does not check revocation either: `newTLSMethod`
(`internal/core/eap/eap_tls.go`) sets
`ClientAuth: tls.RequireAndVerifyClientCert`, and Go's verification of a client
chain performs no revocation check.

**Ze already requests the evidence and discards it.** Go's TLS client sets
`ocspStapling: true` unconditionally (`crypto/tls`), so every handshake asks the
peer for a stapled OCSP response, and the response is stored and never read.

### The decision this spec must put to the owner before implementation

**What does Ze do when no revocation information is available at all?** The peer
staples nothing, or the responder cannot be reached.

| Option | Consequence |
|--------|-------------|
| Refuse the connection | Conformant with Section 5.4-3 read literally. Breaks against every peer that publishes no revocation information, which today includes strongSwan in `test/interop-ipsec/scenarios/eap-tls13` AND Ze's own server |
| Allow the connection | Works against every peer. A revoked certificate authenticates whenever the check cannot run |
| Operator-settable, with one of the above as the default | The default is still a decision, and it is the one an operator inherits on upgrade |

**Section 5.4-3 is fail-closed as written**, and that is why the question is
sharp. Because Go always requests status, Ze *is* "using Certificate Status
Requests" in the RFC's sense, so the literal reading tells Ze to treat a
certificate with no valid status as invalid and abort.

### What the work covers

| Part | Note |
|------|------|
| Consume the stapled response Ze already receives | The cheapest increment. Still needs an OCSP response parser |
| Intermediate certificates | Section 5.4-1 binds every certificate except the trust anchor. Go's `unmarshalCertificate` skips extensions on every entry after the leaf ("This library only supports OCSP and SCT for leaf certificates"), so a staple can never satisfy 5.4-1. Intermediates need a CRL or an OCSP client |
| A revocation parser | `golang.org/x/crypto/ocsp` is NOT vendored, and nothing in `internal/` or `pkg/` calls `ParseRevocationList`. Vendoring it is a `ai/rules/repo-maintenance.md` decision |
| A staple source for Ze's own server | `newTLSMethod` builds its certificate with `tls.X509KeyPair`, leaving `OCSPStaple` nil, and Go gates stapling on that field. So Section 5.4-2 needs the operator to supply a staple, or Ze to fetch and refresh one |
| Section 5.4-4, re-check after the tunnel is up | Its own subsystem: a connectivity hook, a timer, and a teardown path. Decide whether it is in scope |

### What must not happen

**Do not classify any of the five `{gap}` or `{not-applicable}` to make a gate
pass.** `ai/rules/rfc-compliance.md` reserves that to the owner, and the ruling
above defers the WORK, not the honesty of the ledger.
