# RFC 7627 - Transport Layer Security (TLS) Session Hash and Extended Master Secret Extension

No row in the public ledger. Every requirement this repository extracted from RFC 7627, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 17 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 17 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 17 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 17 | of 27 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (backlog), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 17 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 17 of 17 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 17 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (backlog) |
| Requirements | 27 |
| Gated MUST-level | 17 |
| Obligations that bind Ze | 17 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 17 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7627.md` |
| Requirement shard | `rfc/requirements/rfc7627.md` |
| RFC text | `rfc/full/rfc7627.txt` |

## Enrolment

Not enrolled (backlog, the requirements have not been extracted from the document yet; this is work owed rather than a decision): TLS Session Hash and Extended Master Secret Extension. Summary written 2026-08-24 against rfc/full/rfc7627.txt, which was fetched the same day. It declares 27 requirements: 17 MUST-level over 5 sections, 9 at SHOULD level (8 SHOULD / SHOULD NOT plus one NOT RECOMMENDED) and 1 MAY. It is NOT enrolled because not one of the 17 has a producer inside Ze, so neither route to enrolment is an implementer's to take: there is no Ze function a positive and a negative test could tag, and annotating the remainder is a conformance judgement ai/rules/rfc-compliance.md reserves to the owner. WHERE THE OBLIGATIONS ARE DISCHARGED. Go's crypto/tls owns the extension end to end. Its client sets extendedMasterSecret on every ClientHello it builds (makeClientHello, crypto/tls/handshake_client.go), clearing it only for an ECH inner hello; its server copies the client's bit into the ServerHello (serverHandshakeState.processClientHello, crypto/tls/handshake_server.go); the negotiated result lands on Conn.extMasterSecret; and the Section 5.3 abbreviated-handshake mismatch rules are enforced in clientHandshakeState.processServerHello and serverHandshakeState.checkForResumption. Ze holds no ClientHello or ServerHello encoder and no master-secret derivation, so RFC7627-5.1-1, 5.2-1, 5.2-2, 5.2-4, 5.2-6, 5.3-3, 5.3-4, 5.3-6, 5.3-8, 5.3-9, 5.3-10 and 6.4-2 have no Ze-side producer at all. The Section 5.4 downgrade containment (RFC7627-5.4-1 to 5.4-4 and 5.4-6) is enforced below Ze too: Conn.connectionStateLocked selects noEKMBecauseNoEMS on `c.vers != VersionTLS13 && !c.extMasterSecret` (Conn.connectionStateLocked, crypto/tls/conn.go), which is the MUST NOT of RFC7627-5.4-1 executed by the library rather than by its caller. Ze sets no tls.Config.Renegotiation and computes no tls-unique -- the sha256(TLSUnique) fallback was removed from tlsMethod.deriveMSK -- and the authenticator config sets SessionTicketsDisabled so it offers no abbreviated handshake to resume (newTLSMethod, internal/component/ike/eap/eap_tls.go). WHAT ZE ACTUALLY PRODUCES is the refusal path, and it is two functions in internal/component/ike/eap/eap_tls.go: exportEAPTLSMSK returns the crypto/tls error rather than a zero MSK, and eapTLS12ExportRefused names the peer, the negotiated version, RFC 7627 and the operator's three remedies (move the peer to TLS 1.3 per RFC 9190, add RFC 7627 to its TLS 1.2 stack, or configure another EAP method). RFC 5216 Section 2.3 defines the EAP-TLS MSK as that export, so a TLS 1.2 peer whose session carries no extended master secret cannot authenticate. strongSwan 5.9.14 reaches that state by DEFAULT rather than by limitation: charon ships version_max = 1.2 and negotiates no RFC 7627, while charon.tls.version_max = 1.3 on the same build reaches an established SA (test/interop-ipsec/scenarios/eap-tls13/strongswan.conf, and the failing counterpart is test/interop-ipsec/scenarios/eap-tls). TWO FACTS AN OWNER RULING HAS TO ABSORB. First, RFC 7627 IS OBSOLETE. RFC 9846 (The Transport Layer Security (TLS) Protocol Version 1.3, July 2026) obsoletes RFC 5077, 5246, 6961, 7627, 8422 and 8446; its Appendix D renames the extension to Extended Main Secret and the code point to extended_main_secret, and leaves the Section 4 PRF label unchanged for compatibility; its Appendix E states the only obligation it adds about the extension, a SHOULD that an implementation supporting both TLS 1.3 and earlier versions indicate the use of the Extended Main Secret extension in its APIs whenever TLS 1.3 is used. ai/rules/rfc-compliance.md says the lineage that matters runs FORWARD, so the document that states what Ze owes today is RFC 9846, and rfc/full/rfc9846.txt is not in this repository. Enrolling rfc7627 would gate a superseded text. Second, the tlsunsafeekm override is GONE from this checkout, as of the toolchain bump on 2026-08-25. go.mod pins `toolchain go1.27.0`, whose internal/godebugs/table.go carries `{Name: "tlsunsafeekm", Removed: 27, Old: one}`, so a process that sets it to its old value raises a fatal error before main() rather than getting the unsafe export. crypto/tls agrees at the other end: noEKMBecauseNoEMS (crypto/tls/prf.go) now returns the bare sentence with no override clause at all. So the claim cmd/ze/main.go and the RFC 5216 row of docs/features/rfc-status.md both make, that no override remains, is true of what this tree builds; it was false while the toolchain was 1.26. Escalated for a scoping ruling, per the route rfc1035 and rfc9190 took.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7627.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 17 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **17** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (17):** [`RFC7627-5.1-1`](#rfc7627-5.1-1), [`RFC7627-5.2-1`](#rfc7627-5.2-1), [`RFC7627-5.2-2`](#rfc7627-5.2-2), [`RFC7627-5.2-4`](#rfc7627-5.2-4), [`RFC7627-5.2-6`](#rfc7627-5.2-6), [`RFC7627-5.3-3`](#rfc7627-5.3-3), [`RFC7627-5.3-4`](#rfc7627-5.3-4), [`RFC7627-5.3-6`](#rfc7627-5.3-6), [`RFC7627-5.3-8`](#rfc7627-5.3-8), [`RFC7627-5.3-9`](#rfc7627-5.3-9), [`RFC7627-5.3-10`](#rfc7627-5.3-10), [`RFC7627-5.4-1`](#rfc7627-5.4-1), [`RFC7627-5.4-2`](#rfc7627-5.4-2), [`RFC7627-5.4-3`](#rfc7627-5.4-3), [`RFC7627-5.4-4`](#rfc7627-5.4-4), [`RFC7627-5.4-6`](#rfc7627-5.4-6), [`RFC7627-6.4-2`](#rfc7627-6.4-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7627-5.1-1` | "If the client and server agree on this extension and a full handshake takes place, both client and server MUST use the extended master secret derivation algorithm, as defined in Section 4" (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-1` | "In all handshakes, a client implementing this document MUST send the "extended_master_secret" extension in its ClientHello" (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-2` | "If a server implementing this document receives the "extended_master_secret" extension, it MUST include the extension in its ServerHello message" (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-4` | A server that receives a ClientHello without the extension and chooses to continue the handshake "MUST NOT include the extension in the ServerHello" (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-6` | "If the client and server choose to continue a full handshake without the extension, they MUST use the standard master secret derivation for the new session" (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-3` | "When offering an abbreviated handshake, the client MUST send the "extended_master_secret" extension in its ClientHello" (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-4` | Where the original session did not use the extension but the new ClientHello contains it, "the server MUST NOT perform the abbreviated handshake" (§5.3) | MUST NOT | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-6` | Where the original session used the extension but the new ClientHello does not contain it, "the server MUST abort the abbreviated handshake" (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-8` | Where the new ClientHello contains the extension and the server chooses to continue the handshake, "the server MUST include the "extended_master_secret" extension in its ServerHello message" (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-9` | Where the original session did not use the extension but the new ServerHello contains it, "the client MUST abort the handshake" (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-10` | Where the original session used the extension but the new ServerHello does not contain it, "the client MUST abort the handshake" (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-1` | After a full handshake continued without the extension, "the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication" (§5.4) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-2` | After a full handshake continued without the extension, "it MUST disable [RFC5705] and any Extensible Authentication Protocol (EAP) relying on compound authentication" (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-3` | After an abbreviated handshake resuming a session without the extended master secret, "the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication" (§5.4) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-4` | After an abbreviated handshake resuming a session without the extended master secret, "the client MUST disable renegotiation and any use of the "tls-unique" channel binding [RFC5929] on the current connection" (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-6` | Where the original session uses an extended master secret but a hello of the abbreviated handshake omits the extension, "this document recommends that the client and server MUST abort such handshakes" (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-6.4-2` | "If they choose to support SSL 3.0, the resulting sessions MUST use the legacy master secret computation" (§6.4) | MUST | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-4-1` | "Clients and servers SHOULD NOT accept handshakes that do not use the extended master secret, especially if they rely on features like compound authentication that fall into the vulnerable cases described in Section 6.1" (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-3` | "If the server receives a ClientHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy clients" (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-5` | "If a client receives a ServerHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy servers" (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-1` | "The client SHOULD NOT offer an abbreviated handshake to resume a session that does not use an extended master secret" (§5.3) | SHOULD NOT | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-2` | Instead of offering an abbreviated handshake for a session without an extended master secret, the client "SHOULD offer a full handshake" (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-5` | Where the original session did not use the extension but the new ClientHello contains it, the server "SHOULD continue with a full handshake (as described in Section 5.2) to negotiate a new session" (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-7` | "If neither the original session nor the new ClientHello uses the extension, the server SHOULD abort the handshake" (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-6.4-1` | "Clients and servers implementing this document SHOULD refuse SSL 3.0 handshakes" (§6.4) | SHOULD | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-6.2-1` | For the session hash, "hash functions such as MD5 or SHA1 are NOT RECOMMENDED" (§6.2) | NOT RECOMMENDED | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-5` | Where the original session uses an extended master secret but a hello of the abbreviated handshake omits the extension, "it MAY be safe to continue the abbreviated handshake since it is protected by the extended master secret of the original session" (§5.4) | MAY | 5.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7627-5.1-1`](#rfc7627-5.1-1) "If the client and server agree on this extension and a full handshake takes place, both client and server MUST use the extended master secret derivation algorithm, as defined in Section 4" (§5.1) | no test | no test carries this requirement id |
| [`RFC7627-5.2-1`](#rfc7627-5.2-1) "In all handshakes, a client implementing this document MUST send the "extended_master_secret" extension in its ClientHello" (§5.2) | no test | no test carries this requirement id |
| [`RFC7627-5.2-2`](#rfc7627-5.2-2) "If a server implementing this document receives the "extended_master_secret" extension, it MUST include the extension in its ServerHello message" (§5.2) | no test | no test carries this requirement id |
| [`RFC7627-5.2-4`](#rfc7627-5.2-4) A server that receives a ClientHello without the extension and chooses to continue the handshake "MUST NOT include the extension in the ServerHello" (§5.2) | no test | no test carries this requirement id |
| [`RFC7627-5.2-6`](#rfc7627-5.2-6) "If the client and server choose to continue a full handshake without the extension, they MUST use the standard master secret derivation for the new session" (§5.2) | no test | no test carries this requirement id |
| [`RFC7627-5.3-3`](#rfc7627-5.3-3) "When offering an abbreviated handshake, the client MUST send the "extended_master_secret" extension in its ClientHello" (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-4`](#rfc7627-5.3-4) Where the original session did not use the extension but the new ClientHello contains it, "the server MUST NOT perform the abbreviated handshake" (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-6`](#rfc7627-5.3-6) Where the original session used the extension but the new ClientHello does not contain it, "the server MUST abort the abbreviated handshake" (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-8`](#rfc7627-5.3-8) Where the new ClientHello contains the extension and the server chooses to continue the handshake, "the server MUST include the "extended_master_secret" extension in its ServerHello message" (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-9`](#rfc7627-5.3-9) Where the original session did not use the extension but the new ServerHello contains it, "the client MUST abort the handshake" (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-10`](#rfc7627-5.3-10) Where the original session used the extension but the new ServerHello does not contain it, "the client MUST abort the handshake" (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.4-1`](#rfc7627-5.4-1) After a full handshake continued without the extension, "the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication" (§5.4) | no test | no test carries this requirement id |
| [`RFC7627-5.4-2`](#rfc7627-5.4-2) After a full handshake continued without the extension, "it MUST disable [RFC5705] and any Extensible Authentication Protocol (EAP) relying on compound authentication" (§5.4) | no test | no test carries this requirement id |
| [`RFC7627-5.4-3`](#rfc7627-5.4-3) After an abbreviated handshake resuming a session without the extended master secret, "the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication" (§5.4) | no test | no test carries this requirement id |
| [`RFC7627-5.4-4`](#rfc7627-5.4-4) After an abbreviated handshake resuming a session without the extended master secret, "the client MUST disable renegotiation and any use of the "tls-unique" channel binding [RFC5929] on the current connection" (§5.4) | no test | no test carries this requirement id |
| [`RFC7627-5.4-6`](#rfc7627-5.4-6) Where the original session uses an extended master secret but a hello of the abbreviated handshake omits the extension, "this document recommends that the client and server MUST abort such handshakes" (§5.4) | no test | no test carries this requirement id |
| [`RFC7627-6.4-2`](#rfc7627-6.4-2) "If they choose to support SSL 3.0, the resulting sessions MUST use the legacy master secret computation" (§6.4) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7627-5.1-1`](#rfc7627-5.1-1)

"If the client and server agree on this extension and a full handshake takes place, both client and server MUST use the extended master secret derivation algorithm, as defined in Section 4" (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.1-1, so no unit is bound to it.

### [`RFC7627-5.2-1`](#rfc7627-5.2-1)

"In all handshakes, a client implementing this document MUST send the "extended_master_secret" extension in its ClientHello" (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.2-1, so no unit is bound to it.

### [`RFC7627-5.2-2`](#rfc7627-5.2-2)

"If a server implementing this document receives the "extended_master_secret" extension, it MUST include the extension in its ServerHello message" (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.2-2, so no unit is bound to it.

### [`RFC7627-5.2-4`](#rfc7627-5.2-4)

A server that receives a ClientHello without the extension and chooses to continue the handshake "MUST NOT include the extension in the ServerHello" (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.2-4, so no unit is bound to it.

### [`RFC7627-5.2-6`](#rfc7627-5.2-6)

"If the client and server choose to continue a full handshake without the extension, they MUST use the standard master secret derivation for the new session" (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.2-6, so no unit is bound to it.

### [`RFC7627-5.3-3`](#rfc7627-5.3-3)

"When offering an abbreviated handshake, the client MUST send the "extended_master_secret" extension in its ClientHello" (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.3-3, so no unit is bound to it.

### [`RFC7627-5.3-4`](#rfc7627-5.3-4)

Where the original session did not use the extension but the new ClientHello contains it, "the server MUST NOT perform the abbreviated handshake" (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.3-4, so no unit is bound to it.

### [`RFC7627-5.3-6`](#rfc7627-5.3-6)

Where the original session used the extension but the new ClientHello does not contain it, "the server MUST abort the abbreviated handshake" (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.3-6, so no unit is bound to it.

### [`RFC7627-5.3-8`](#rfc7627-5.3-8)

Where the new ClientHello contains the extension and the server chooses to continue the handshake, "the server MUST include the "extended_master_secret" extension in its ServerHello message" (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.3-8, so no unit is bound to it.

### [`RFC7627-5.3-9`](#rfc7627-5.3-9)

Where the original session did not use the extension but the new ServerHello contains it, "the client MUST abort the handshake" (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.3-9, so no unit is bound to it.

### [`RFC7627-5.3-10`](#rfc7627-5.3-10)

Where the original session used the extension but the new ServerHello does not contain it, "the client MUST abort the handshake" (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.3-10, so no unit is bound to it.

### [`RFC7627-5.4-1`](#rfc7627-5.4-1)

After a full handshake continued without the extension, "the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication" (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.4-1, so no unit is bound to it.

### [`RFC7627-5.4-2`](#rfc7627-5.4-2)

After a full handshake continued without the extension, "it MUST disable [RFC5705] and any Extensible Authentication Protocol (EAP) relying on compound authentication" (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.4-2, so no unit is bound to it.

### [`RFC7627-5.4-3`](#rfc7627-5.4-3)

After an abbreviated handshake resuming a session without the extended master secret, "the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication" (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.4-3, so no unit is bound to it.

### [`RFC7627-5.4-4`](#rfc7627-5.4-4)

After an abbreviated handshake resuming a session without the extended master secret, "the client MUST disable renegotiation and any use of the "tls-unique" channel binding [RFC5929] on the current connection" (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.4-4, so no unit is bound to it.

### [`RFC7627-5.4-6`](#rfc7627-5.4-6)

Where the original session uses an extended master secret but a hello of the abbreviated handshake omits the extension, "this document recommends that the client and server MUST abort such handshakes" (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-5.4-6, so no unit is bound to it.

### [`RFC7627-6.4-2`](#rfc7627-6.4-2)

"If they choose to support SSL 3.0, the resulting sessions MUST use the legacy master secret computation" (§6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7627-6.4-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7627, so no reviewer has walked its text sentence by sentence.

## Superseded

RFC 7627 is obsoleted by RFC 9846.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC7627-5.1-1`](#rfc7627-5.1-1) "If the client and server agree on this extension and a full handshake takes place, both client and server MUST use the extended master secret derivation algorithm, as defined in Section 4" (§5.1) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-1`](#rfc7627-5.2-1) "In all handshakes, a client implementing this document MUST send the "extended_master_secret" extension in its ClientHello" (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-2`](#rfc7627-5.2-2) "If a server implementing this document receives the "extended_master_secret" extension, it MUST include the extension in its ServerHello message" (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-4`](#rfc7627-5.2-4) A server that receives a ClientHello without the extension and chooses to continue the handshake "MUST NOT include the extension in the ServerHello" (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-6`](#rfc7627-5.2-6) "If the client and server choose to continue a full handshake without the extension, they MUST use the standard master secret derivation for the new session" (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-3`](#rfc7627-5.3-3) "When offering an abbreviated handshake, the client MUST send the "extended_master_secret" extension in its ClientHello" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-4`](#rfc7627-5.3-4) Where the original session did not use the extension but the new ClientHello contains it, "the server MUST NOT perform the abbreviated handshake" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-6`](#rfc7627-5.3-6) Where the original session used the extension but the new ClientHello does not contain it, "the server MUST abort the abbreviated handshake" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-8`](#rfc7627-5.3-8) Where the new ClientHello contains the extension and the server chooses to continue the handshake, "the server MUST include the "extended_master_secret" extension in its ServerHello message" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-9`](#rfc7627-5.3-9) Where the original session did not use the extension but the new ServerHello contains it, "the client MUST abort the handshake" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-10`](#rfc7627-5.3-10) Where the original session used the extension but the new ServerHello does not contain it, "the client MUST abort the handshake" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-1`](#rfc7627-5.4-1) After a full handshake continued without the extension, "the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication" (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-2`](#rfc7627-5.4-2) After a full handshake continued without the extension, "it MUST disable [RFC5705] and any Extensible Authentication Protocol (EAP) relying on compound authentication" (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-3`](#rfc7627-5.4-3) After an abbreviated handshake resuming a session without the extended master secret, "the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication" (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-4`](#rfc7627-5.4-4) After an abbreviated handshake resuming a session without the extended master secret, "the client MUST disable renegotiation and any use of the "tls-unique" channel binding [RFC5929] on the current connection" (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-6`](#rfc7627-5.4-6) Where the original session uses an extended master secret but a hello of the abbreviated handshake omits the extension, "this document recommends that the client and server MUST abort such handshakes" (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-6.4-2`](#rfc7627-6.4-2) "If they choose to support SSL 3.0, the resulting sessions MUST use the legacy master secret computation" (§6.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-4-1`](#rfc7627-4-1) "Clients and servers SHOULD NOT accept handshakes that do not use the extended master secret, especially if they rely on features like compound authentication that fall into the vulnerable cases described in Section 6.1" (§4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-3`](#rfc7627-5.2-3) "If the server receives a ClientHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy clients" (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-5`](#rfc7627-5.2-5) "If a client receives a ServerHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy servers" (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-1`](#rfc7627-5.3-1) "The client SHOULD NOT offer an abbreviated handshake to resume a session that does not use an extended master secret" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-2`](#rfc7627-5.3-2) Instead of offering an abbreviated handshake for a session without an extended master secret, the client "SHOULD offer a full handshake" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-5`](#rfc7627-5.3-5) Where the original session did not use the extension but the new ClientHello contains it, the server "SHOULD continue with a full handshake (as described in Section 5.2) to negotiate a new session" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-7`](#rfc7627-5.3-7) "If neither the original session nor the new ClientHello uses the extension, the server SHOULD abort the handshake" (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-6.4-1`](#rfc7627-6.4-1) "Clients and servers implementing this document SHOULD refuse SSL 3.0 handshakes" (§6.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-6.2-1`](#rfc7627-6.2-1) For the session hash, "hash functions such as MD5 or SHA1 are NOT RECOMMENDED" (§6.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-5`](#rfc7627-5.4-5) Where the original session uses an extended master secret but a hello of the abbreviated handshake omits the extension, "it MAY be safe to continue the abbreviated handshake since it is protected by the extended master secret of the original session" (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
