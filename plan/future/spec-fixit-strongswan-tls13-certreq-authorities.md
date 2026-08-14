# Spec: fixit-strongswan-tls13-certreq-authorities -- two upstream strongSwan defects Ze works around, and the reports that must be filed

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; the spec already says the shard exists only if work is deferred out of it. Create `plan/deferrals/fixit-strongswan-tls13-certreq-authorities.md` on the first deferral) | <!-- doc-links: ignore (shard owed only on the first deferral; none was made) -->
| Updated | 2026-08-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Deferral holder created on 2026-08-02 while the rfcgate-1b RFC 7296 pilot spec was
closing. The finding is not the pilot's work and had no home.

## Provenance

Reclassified as an improvement on 2026-08-14 at Thomas's instruction and moved
from `plan/` to `plan/future/`. Reason: Ze is conformant, the peer is not, the
workaround is in place and cross-referenced from both sites that pay for it.
The deliverable is filing two upstream reports, which changes nothing in Ze.

## Task

**strongSwan 5.9.14 carries two defects in one function, and Ze already pays for both with a
test-lab workaround. This spec owns REPORTING them upstream and removing the workaround when
upstream fixes them.**

Ze is conformant. Go's `crypto/tls` is conformant. The peer is not. The workaround is
correct and stays until upstream moves, so nothing here is a Ze protocol change.

### The two defects

Both live in `write_certificate_authorities` (`src/libtls/tls_server.c`, strongSwan 5.9.14),
the function that builds the TLS 1.3 `CertificateRequest` `certificate_authorities`
extension.

| # | Defect | Consequence |
|---|--------|-------------|
| 1 | The trust-anchor enumerator is called with a hardcoded `KEY_RSA` key type. `certificate_matches` (`src/libstrongswan/credentials/certificates/certificate.c`) returns false for any certificate whose public key type differs from the requested one | An ECDSA CA is NEVER enumerable, however it is configured. Setting `cacerts` on the remote section does not help: the anchor is already loaded and trusted, and the filter rejects it on key type alone |
| 2 | The extension body is written unconditionally, with no guard on the enumerated list being empty | With an empty list charon still emits the extension, as the three-field encoding `002f 0002 0000`: extension type 47, body length 2, authorities list length 0 |

Defect 2 is the wire-visible violation, and defect 1 is what makes it reachable in practice.
Defect 2 stands on its own: any deployment whose anchors all fail the enumerator filter
emits the same malformed extension.

### Why the empty list is malformed, not merely unusual

RFC 8446 Section 4.2.4 declares the field as `DistinguishedName authorities<3..2^16-1>`. The
lower bound is 3, so the shortest legal encoding of the list is 3 octets and a zero-length
list has no legal encoding at all.

Go refuses it in `certificateRequestMsgTLS13.unmarshal` (`crypto/tls`), whose parse of the
extension fails when the length-prefixed value is empty. Ze surfaces that as
`local error: tls: error decoding message`, and the IKE SA never leaves CONNECTING.

### What Ze does today, and why that is the right answer

Scenario `06-eap-tls13` uses an RSA CA where every other scenario uses `prime256v1`. The
leaf keys stay `prime256v1`, so the scenario still exercises the same signature algorithms
as the rest of the lab. Only the anchor changed.

The reasoning is already recorded at the two sites that carry the workaround, in detail, and
this spec does not restate it:

- `test/ipsec-interop/scenarios/06-eap-tls13/pki/gen-pki.sh` -- the header comment states the
  single load-bearing difference and why it is a property of the peer, not of Ze.
- `test/ipsec-interop/scenarios/06-eap-tls13/strongswan.conf` -- the header comment carries
  the measurement of 2026-08-01: with the shared EC CA charon logged `sending TLS cert
  request` zero times and the SA never left CONNECTING; with the RSA CA it logs it once and
  the SA reaches ESTABLISHED.

`send_certreq_authorities` is deliberately left at charon's shipped default of `yes`, so the
scenario proves an exchange a stock strongSwan performs.

### What this spec must deliver

The precedent is `plan/spec-ipsec-opaque-selector-port-mask.md`, which carries a
`vishvananda/netlink` defect, its minimal patch, and the obligation to send it upstream. This
spec carries the same three things for strongSwan.

| Deliverable | Why it is here and not elsewhere |
|-------------|----------------------------------|
| Two upstream reports, filed | A defect nobody reported is a defect that stays. `ai/rules/completion.md`: recording is not addressing |
| The workaround kept and cross-referenced to the reports | A future reader must be able to reach the report from the code that pays for the bug |
| A removal condition, stated | Without one the RSA CA becomes permanent and its reason is forgotten |

## Ready-to-file upstream report text

Written as prose because `ai/rules/spec-no-code.md` forbids code in a spec. File as TWO
issues: they have different blast radii, and defect 2 is fixable without defect 1.

### Issue 1 -- `write_certificate_authorities` enumerates RSA trust anchors only

Title: `TLS 1.3 CertificateRequest omits every non-RSA trust anchor (libtls, hardcoded KEY_RSA)`

Body, in five parts:

| Part | Content |
|------|---------|
| Summary | In `write_certificate_authorities` (`src/libtls/tls_server.c`), the credential-manager certificate enumerator is created with the key type fixed to `KEY_RSA`. `certificate_matches` rejects any certificate whose public key type differs from the requested one, so an ECDSA (or Ed25519) trust anchor is never placed in the `certificate_authorities` list |
| Version | 5.9.14, as packaged by Alpine 3.21 (`test/ipsec-interop/Dockerfile.strongswan` installs the distribution `strongswan` package) |
| Reproduction | Configure charon as an EAP-TLS server with `version_min` and `version_max` both 1.3, and a single ECDSA CA as the trust anchor. Connect any TLS 1.3 client. charon never logs `sending TLS cert request for '<DN>'` |
| Expected | The enumerator is unfiltered by key type, or is run once per key type, so any configured trust anchor of any key type reaches the list |
| Actual | The list is empty. With an RSA CA and no other change, charon logs the line once and the handshake completes |

Add that the same call site is the antecedent of issue 2: an empty list is what issue 2's
missing guard then emits.

### Issue 2 -- an empty `certificate_authorities` extension is emitted, violating RFC 8446

Title: `TLS 1.3 CertificateRequest emits a zero-length certificate_authorities extension (RFC 8446 4.2.4 minimum is 3)`

Body, in five parts:

| Part | Content |
|------|---------|
| Summary | `write_certificate_authorities` writes the extension body unconditionally. When the enumeration yields nothing the extension is still emitted with a zero-length authorities list, encoded `002f 0002 0000`. RFC 8446 Section 4.2.4 declares `DistinguishedName authorities<3..2^16-1>`, so the shortest legal list is 3 octets and a zero-length list is not encodable |
| Version | 5.9.14 |
| Reproduction | Any configuration in which the enumeration yields no anchor. Issue 1 is the easiest route: an ECDSA-only trust anchor. Capture the `CertificateRequest` and read the extension |
| Impact | A conforming peer rejects the message. Go's `crypto/tls` fails the parse in `certificateRequestMsgTLS13.unmarshal` and answers `decode_error`, so the handshake dies at the first `CertificateRequest` rather than degrading |
| Expected | Omit the extension entirely when the list is empty. RFC 8446 makes `certificate_authorities` optional, so omission is both legal and the natural fix, and it needs no change to issue 1 |

State that the two issues are independent: fixing issue 2 alone converts a fatal handshake
failure into a working handshake with no CA hint, which is a correct TLS 1.3 exchange.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/completion.md` - recording a defect is not addressing it
  → Constraint: a defect found while doing something else is the reason you are now the one
    who reports it. The report is the action; this spec is where it is tracked, not a
    substitute for it.
- [ ] `ai/rules/writing.md` - claims about another project need a durable source
  → Constraint: cite the upstream source file and symbol, and label anything not read.
- [ ] `plan/spec-ipsec-opaque-selector-port-mask.md` - the precedent for an upstream report
  → Decision: a spec that carries an upstream defect also carries the minimal patch shape and
    the obligation to send it, and no phase may drop that obligation.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8446.md` - TLS 1.3, Section 4.2.4 `certificate_authorities` <!-- doc-links: ignore (no RFC 8446 summary in this tree) -->
  → Constraint: `DistinguishedName authorities<3..2^16-1>`. A zero-length list has no legal
    encoding. Create the summary if it is absent; do not cite Section 4.2.4 from memory.

**Key insights:**
- Ze needs no protocol change. The defects are the peer's, and Ze's rejection of the
  malformed extension is Go's stdlib behaving correctly.
- The workaround is a PKI choice in one scenario, not a code path, so removing it is a
  one-file change once upstream ships a fix.

## Current Behavior (MANDATORY)

**Source files read on 2026-08-02:**

- [ ] `test/ipsec-interop/scenarios/06-eap-tls13/pki/gen-pki.sh` - generates a scenario-local
  PKI whose CA key is RSA; the header comment states why, and the leaf keys stay `prime256v1`
- [ ] `test/ipsec-interop/Dockerfile.strongswan` - the peer is the Alpine 3.21 distribution
  `strongswan` package, run as `charon` in the foreground

**Behavior to preserve:** scenario `06-eap-tls13` must keep passing at charon's shipped
`send_certreq_authorities = yes`, and the leaf keys must stay `prime256v1` so the scenario
keeps exercising the lab's signature algorithms. The two header comments must not be deleted
while the workaround stands.

**Behavior to change:** none in Ze, until upstream ships a fix. Then the scenario-local PKI
is deleted and the scenario returns to the shared `test/ipsec-interop/pki/gen-pki.sh`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

A TLS 1.3 `CertificateRequest` arriving at Ze's EAP-TLS client from charon, inside IKE_AUTH.

### Transformation Path

1. charon builds the `certificate_authorities` extension in `write_certificate_authorities`
   (`src/libtls/tls_server.c`), enumerating trust anchors filtered to `KEY_RSA`.
2. With no anchor enumerated, the extension is written anyway with a zero-length list.
3. The record reaches Ze's EAP-TLS transport and is handed to Go's `crypto/tls`.
4. `certificateRequestMsgTLS13.unmarshal` fails the parse and Go answers `decode_error`.
5. Ze reports `local error: tls: error decoding message` and the IKE SA stays CONNECTING.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| charon ↔ Ze EAP-TLS | TLS 1.3 records carried in EAP-TLS inside IKE_AUTH | Yes, by scenario `06-eap-tls13` |
| Ze EAP-TLS ↔ Go `crypto/tls` | the stdlib handshake, which is where the rejection happens | Yes |
| Ze ↔ upstream strongSwan | two issue reports, and a patch if one is offered | No |

### Integration Points

- `test/ipsec-interop/scenarios/06-eap-tls13/` - the only site that pays for the defects.
- `docs/features/` - no user-facing claim rests on this today; confirm at design time.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Both defects are still present on strongSwan `master` | Read at 5.9.14 only, which is what the lab runs. `master` was NOT read | The report is stale and must name the version range that is actually affected | Read `write_certificate_authorities` on `master` before filing | unvalidated |
| A-2 | No upstream issue or merge request already covers either defect | NOT checked. No search was performed | The work is a comment on an existing report, not a new one | Search the strongSwan issue tracker for `certificate_authorities`, `KEY_RSA` and `write_certificate_authorities` before filing | unvalidated |
| A-3 | Ze's rejection is Go's stdlib, not Ze code | The error text `local error: tls: error decoding message` is a stdlib string and Ze runs the stdlib handshake | Ze would own a parser bug of its own, which changes the whole spec | Read the Ze EAP-TLS transport and confirm no Ze code parses the extension | unvalidated |
| A-4 | Omitting the extension when empty is acceptable to upstream | RFC 8446 makes the extension optional | The fix upstream chooses is different and the report's Expected section is wrong | State the constraint and let upstream choose the fix; do not prescribe | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The RSA CA becomes permanent and its reason is forgotten, so a later reader "tidies" the scenario back to the shared PKI and reopens a red with no explanation | A diff that deletes `gen-pki.sh` without naming this spec | Both header comments already carry the reason; add this spec's path to them so the reader reaches the report |
| R-2 | The lab's strongSwan version floats, because the Dockerfile installs the Alpine distribution package rather than a pinned version. A future base-image bump could fix or change the behavior silently | Scenario 06 starts passing with an EC CA, or starts failing differently | Pin the observed version in the scenario comment and re-measure when the base image moves |
| R-3 | Filing is deferred indefinitely because it is not code | The spec sits at `skeleton` across several sessions with no report URL recorded | The report IS the deliverable. Record the URL in the learned summary, exactly as the netlink precedent requires |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the daemon. The whole surface is one interop scenario's PKI plus two upstream reports |
| How is it reverted? | Single commit. The scenario-local PKI is additive |
| Who else touches this path? | `spec-fixit-eap-tls-clienthello-race` owns the EAP-TLS client behavior for scenario 04; it does not touch the CA key type or the extension |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| charon sends a TLS 1.3 `CertificateRequest` | → | Ze's EAP-TLS client completes the handshake | `test/ipsec-interop/scenarios/06-eap-tls13/check.py`, which asserts charon's `sending TLS cert request` line and an ESTABLISHED SA |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The strongSwan issue tracker searched for prior art on both defects | The result is recorded, with the query and the date, whether or not anything was found |
| AC-2 | `write_certificate_authorities` read on strongSwan `master` | The affected version range is stated from what was read, not assumed from 5.9.14 |
| AC-3 | Both defects filed upstream (or an existing report identified) | Two URLs recorded in the learned summary |
| AC-4 | The two scenario header comments | Each names this spec, so a reader reaches the upstream report from the workaround |
| AC-5 | Upstream ships a fix in a version the lab can install | The scenario-local PKI is deleted, the scenario returns to the shared PKI, and it still passes |

AC-5 is the removal condition and is expected to outlive the other four. It is recorded so
the workaround has a stated end, not so this spec waits on it.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| none | - | No Ze code changes, so no unit test is owed. The evidence is the interop scenario and the upstream reports | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `06-eap-tls13` | `test/ipsec-interop/scenarios/06-eap-tls13/check.py` | An EAP-TLS session over TLS 1.3 against a stock strongSwan reaches ESTABLISHED | passing today, via the workaround |

No `.ci` is owed. This spec changes no daemon code: the whole surface is one scenario's PKI,
two header comments, and two upstream reports. `check.py` is the driving surface.

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `06-eap-tls13` | `test/ipsec-interop/scenarios/` | strongSwan 5.9.14 | TLS 1.3 EAP-TLS interoperates once the peer's anchor is enumerable | passing |

## Files to Modify

No daemon code changes. Written as a table so the list is not read as feature code the spec
does not touch.

| File | Change |
|------|--------|
| `test/ipsec-interop/scenarios/06-eap-tls13/pki/gen-pki.sh` | Add this spec's path to the header comment, so the workaround points at the upstream report |
| `test/ipsec-interop/scenarios/06-eap-tls13/strongswan.conf` | Same |

The Ze EAP-TLS transport is read at design time to confirm A-3 (that no Ze code parses the
extension, and the rejection is the Go stdlib). It is Required Reading, not a change.

## Files to Create

| File | Content |
|------|---------|
| `docs/labs/ipsec-interop.md` | The lab page the l2tp and pppoe labs already have, carrying the peer-defect note and this spec's path. Optional: fold into the header comments instead if the design phase judges a page unwarranted | <!-- doc-links: ignore (page this spec proposes; not written) -->
| `plan/deferrals/fixit-strongswan-tls13-certreq-authorities.md` | This spec's shard, if any work is deferred out of it | <!-- doc-links: ignore (shard owed only on the first deferral; none was made) -->

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- confirm the scenario is the wiring, and that it
   still passes.
   - Verify: `06-eap-tls13` green, and the two header comments present.
2. **Phase: Verify the report before filing it** -- resolve A-1, A-2 and A-3.
   - Read `write_certificate_authorities` on `master`, search the tracker, read Ze's EAP-TLS
     transport.
   - Verify: each assumption `confirmed` or `broken`, with the evidence recorded.
3. **Phase: File** -- submit both issues using the text above, adjusted for what phase 2 found.
   - Verify: two URLs recorded.
4. **Phase: Cross-reference** -- add the spec path to the two header comments.
5. **Phase: Removal condition** -- record AC-5 and its trigger in the learned summary, then
   close. Do not hold the spec open waiting on upstream.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| No fabrication | Every claim about strongSwan names the file and the function it was read in, and anything not read is labelled. `ai/rules/evidence.md` |
| Comparison honesty | The report states the inspected version and does not generalize beyond it |
| Removal condition | AC-5 exists and is reachable, so the workaround is not permanent by default |

## Known Limitations

- Ze cannot fix the peer. This spec's only lever on the defects is the report.
- The lab installs the distribution `strongswan` package rather than a pinned version, so
  the observed behavior is tied to whatever Alpine 3.21 ships. R-2 covers it.

## RFC Documentation (Scope: protocol)

RFC 8446 Section 4.2.4 governs the extension. The violation is the peer's, so no Ze code
gains an RFC comment and no `RFC8446-*` requirement id is claimed by this spec.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-4 demonstrated. AC-5 recorded with its trigger
- [ ] `make ze-verify` passes
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: the evidence is the interop
      scenario, which exists and passes)
