# RFC 7627 - Transport Layer Security (TLS) Session Hash and Extended Master Secret Extension

No entry in the public support list. Every requirement listed for RFC 7627, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 2 | of 17 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 17 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 17 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 17 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 4 | 4 of 4 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 15 | of 17 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 17 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 17 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 17 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 17 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 17 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 17 | of 27 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (backlog), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 17 mandatory requirements: they add up to 17. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | No entry in the public support list |
| Automatic checks | Excluded (backlog) |
| Requirements | 27 |
| Mandatory requirements (MUST level), not checked | 17 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 15 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tests linked to requirements | 4 |
| Recorded audit verdicts | 0 |
| Fault test records | 4 |
| Summary | `rfc/short/rfc7627.md` |
| Requirement table | `rfc/requirements/rfc7627.md` |
| RFC text | `rfc/full/rfc7627.txt` |

## Automatic checks

Excluded from automatic checks (backlog, the requirements have not been extracted from the document yet; this is work owed rather than a decision): TLS Session Hash and Extended Master Secret Extension. Summary written 2026-08-24 against rfc/full/rfc7627.txt, which was fetched the same day. It declares 27 requirements: 17 MUST-level over 5 sections, 9 at SHOULD level (8 SHOULD / SHOULD NOT plus one NOT RECOMMENDED) and 1 MAY. It is NOT enrolled because not one of the 17 has a producer inside Ze, so neither route to enrolment is an implementer's to take: there is no Ze function a positive and a negative test could tag, and annotating the remainder is a conformance judgement ai/rules/rfc-compliance.md reserves to the owner. WHERE THE OBLIGATIONS ARE DISCHARGED. Go's crypto/tls owns the extension end to end. Its client sets extendedMasterSecret on every ClientHello it builds (makeClientHello, crypto/tls/handshake_client.go), clearing it only for an ECH inner hello; its server copies the client's bit into the ServerHello (serverHandshakeState.processClientHello, crypto/tls/handshake_server.go); the negotiated result lands on Conn.extMasterSecret; and the Section 5.3 abbreviated-handshake mismatch rules are enforced in clientHandshakeState.processServerHello and serverHandshakeState.checkForResumption. Ze holds no ClientHello or ServerHello encoder and no master-secret derivation, so RFC7627-5.1-1, 5.2-1, 5.2-2, 5.2-4, 5.2-6, 5.3-3, 5.3-4, 5.3-6, 5.3-8, 5.3-9, 5.3-10 and 6.4-2 have no Ze-side producer at all. The Section 5.4 downgrade containment (RFC7627-5.4-1 to 5.4-4 and 5.4-6) is enforced below Ze too: Conn.connectionStateLocked selects noEKMBecauseNoEMS on `c.vers != VersionTLS13 && !c.extMasterSecret` (Conn.connectionStateLocked, crypto/tls/conn.go), which is the MUST NOT of RFC7627-5.4-1 executed by the library rather than by its caller. Ze sets no tls.Config.Renegotiation and computes no tls-unique -- the sha256(TLSUnique) fallback was removed from tlsMethod.deriveKeys -- and the authenticator now issues a TLS 1.3 session ticket per RFC 9190 Section 2.1.2, whose resumption is a TLS 1.3 PSK exchange and not the RFC 7627 abbreviated handshake this document governs (newTLSMethod, internal/core/eap/eap_tls.go). WHAT ZE ACTUALLY PRODUCES is the refusal path, and it is two functions in internal/core/eap/eap_tls.go: exportEAPTLSKeys returns the crypto/tls error rather than a zero MSK, and eapTLS12ExportRefused names the peer, the negotiated version, RFC 7627 and the operator's three remedies (move the peer to TLS 1.3 per RFC 9190, add RFC 7627 to its TLS 1.2 stack, or configure another EAP method). RFC 5216 Section 2.3 defines the EAP-TLS MSK as that export, so a TLS 1.2 peer whose session carries no extended master secret cannot authenticate. strongSwan 5.9.14 reaches that state by DEFAULT rather than by limitation: charon ships version_max = 1.2 and negotiates no RFC 7627, while charon.tls.version_max = 1.3 on the same build reaches an established SA (test/interop-ipsec/scenarios/eap-tls13/strongswan.conf, and the failing counterpart is test/interop-ipsec/scenarios/eap-tls). TWO FACTS AN OWNER RULING HAS TO ABSORB. First, RFC 7627 IS OBSOLETE. RFC 9846 (The Transport Layer Security (TLS) Protocol Version 1.3, July 2026) obsoletes RFC 5077, 5246, 6961, 7627, 8422 and 8446; its Appendix D renames the extension to Extended Main Secret and the code point to extended_main_secret, and leaves the Section 4 PRF label unchanged for compatibility; its Appendix E states the only obligation it adds about the extension, a SHOULD that an implementation supporting both TLS 1.3 and earlier versions indicate the use of the Extended Main Secret extension in its APIs whenever TLS 1.3 is used. ai/rules/rfc-compliance.md says the lineage that matters runs FORWARD, so the document that states what Ze owes today is RFC 9846, and rfc/full/rfc9846.txt is not in this repository. Enrolling rfc7627 would gate a superseded text. Second, the tlsunsafeekm override is GONE from this checkout, as of the toolchain bump on 2026-08-25. go.mod pins `toolchain go1.27.0`, whose internal/godebugs/table.go carries `{Name: "tlsunsafeekm", Removed: 27, Old: one}`, so a process that sets it to its old value raises a fatal error before main() rather than getting the unsafe export. crypto/tls agrees at the other end: noEKMBecauseNoEMS (crypto/tls/prf.go) now returns the bare sentence with no override clause at all. So the claim cmd/ze/main.go and the RFC 5216 row of docs/features/rfc-status.md both make, that no override remains, is true of what this tree builds; it was false while the toolchain was 1.26. Escalated for a scoping ruling, per the route rfc1035 and rfc9190 took.

## Published support status

No entry in the public support list, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7627.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 15 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **17** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (2):** [`RFC7627-5.4-1`](#rfc7627-5.4-1), [`RFC7627-5.4-2`](#rfc7627-5.4-2)

**No test and no annotation (15):** [`RFC7627-5.1-1`](#rfc7627-5.1-1), [`RFC7627-5.2-1`](#rfc7627-5.2-1), [`RFC7627-5.2-2`](#rfc7627-5.2-2), [`RFC7627-5.2-4`](#rfc7627-5.2-4), [`RFC7627-5.2-6`](#rfc7627-5.2-6), [`RFC7627-5.3-3`](#rfc7627-5.3-3), [`RFC7627-5.3-4`](#rfc7627-5.3-4), [`RFC7627-5.3-6`](#rfc7627-5.3-6), [`RFC7627-5.3-8`](#rfc7627-5.3-8), [`RFC7627-5.3-9`](#rfc7627-5.3-9), [`RFC7627-5.3-10`](#rfc7627-5.3-10), [`RFC7627-5.4-3`](#rfc7627-5.4-3), [`RFC7627-5.4-4`](#rfc7627-5.4-4), [`RFC7627-5.4-6`](#rfc7627-5.4-6), [`RFC7627-6.4-2`](#rfc7627-6.4-2)

## Figures

- **MUST bar**: requirements not reviewed
- **Overview bar**: requirements not reviewed

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 1 |
| Mandatory requirements complete | not complete |
| MUST rows | requirements not reviewed |
| MUST rows kept (the MUST bar) | requirements not reviewed |
| Ze verified | requirements not reviewed |
| OS verified | requirements not reviewed |
| Tests awaiting verification | requirements not reviewed |
| OS testing gap | requirements not reviewed |
| Exclusion unchecked | requirements not reviewed |
| Not assessed | requirements not reviewed |
| Not built | requirements not reviewed |
| MUST rows not owed | requirements not reviewed |
| MUST not owed: feature-declined | requirements not reviewed |
| MUST not owed: linux-limit | requirements not reviewed |
| MUST not owed: not-applicable | requirements not reviewed |
| SHOULD / MAY rows | requirements not reviewed |
| SHOULD verified | requirements not reviewed |
| SHOULD OS verified | requirements not reviewed |
| SHOULD OS testing gap | requirements not reviewed |
| SHOULD not owed | requirements not reviewed |
| SHOULD not owed: feature-declined | requirements not reviewed |
| SHOULD not owed: linux-limit | requirements not reviewed |
| SHOULD not owed: not-applicable | requirements not reviewed |
| SHOULD tests awaiting verification | requirements not reviewed |
| SHOULD exclusion unchecked | requirements not reviewed |
| SHOULD not assessed | requirements not reviewed |
| SHOULD not built | requirements not reviewed |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7627.json](../../../data/rfc/rfc7627.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7627-5.1-1` | If the client and server agree on this extension and a full handshake takes place, both client and server MUST use the extended master secret derivation algorithm, as defined in Section 4. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-1` | In all handshakes, a client implementing this document MUST send the "extended_master_secret" extension in its ClientHello. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-2` | If a server implementing this document receives the "extended_master_secret" extension, it MUST include the extension in its ServerHello message. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-4` | If the server receives a ClientHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy clients. If it chooses to continue the handshake, then it MUST NOT include the extension in the ServerHello. (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-6` | If the client and server choose to continue a full handshake without the extension, they MUST use the standard master secret derivation for the new session. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-3` | When offering an abbreviated handshake, the client MUST send the "extended_master_secret" extension in its ClientHello. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-4` | If the original session did not use the "extended_master_secret" extension but the new ClientHello contains the extension, then the server MUST NOT perform the abbreviated handshake. (§5.3) | MUST NOT | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-6` | If the original session used the "extended_master_secret" extension but the new ClientHello does not contain it, the server MUST abort the abbreviated handshake. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-8` | If the new ClientHello contains the extension and the server chooses to continue the handshake, then the server MUST include the "extended_master_secret" extension in its ServerHello message. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-9` | If the original session did not use the "extended_master_secret" extension but the new ServerHello contains the extension, the client MUST abort the handshake. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-10` | If the original session used the extension but the new ServerHello does not contain the extension, the client MUST abort the handshake. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-1` | If a client or server chooses to continue with a full handshake without the extended master secret extension, then the new session becomes vulnerable to the man-in-the-middle key synchronization attack described in Section 1. Hence, the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication. (§5.4) | MUST NOT | 5.4 | **positive:** `unit/verify` [`TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7627_eap_tls_export_refusal_test.go#L200). **negative:** `interop/nightly` [`checkEAPTLS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L464) |
| `RFC7627-5.4-2` | If a client or server chooses to continue with a full handshake without the extended master secret extension, then the new session becomes vulnerable to the man-in-the-middle key synchronization attack described in Section 1. Hence, the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication. In particular, it MUST disable [RFC5705] and any Extensible Authentication Protocol (EAP) relying on compound authentication [COMPOUND-AUTH]. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7627_eap_tls_export_refusal_test.go#L203). **negative:** `interop/nightly` [`checkEAPTLS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L469) |
| `RFC7627-5.4-3` | If a client or server chooses to continue an abbreviated handshake to resume a session that does not use the extended master secret, then the current connection becomes vulnerable to a man-in-the-middle handshake log synchronization attack as described in Section 1. Hence, the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication. (§5.4) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-4` | If a client or server chooses to continue an abbreviated handshake to resume a session that does not use the extended master secret, then the current connection becomes vulnerable to a man-in-the-middle handshake log synchronization attack as described in Section 1. Hence, the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication. In particular, the client MUST disable renegotiation and any use of the "tls-unique" channel binding [RFC5929] on the current connection. (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-6` | If the original session uses an extended master secret but the ClientHello or ServerHello in the abbreviated handshake does not include the extension, it MAY be safe to continue the abbreviated handshake since it is protected by the extended master secret of the original session. This scenario may occur, for example, when a server that implements this extension establishes a session but the session is subsequently resumed at a different server that does not support the extension. Since such situations are unusual and likely to be the result of transient or inadvertent misconfigurations, this document recommends that the client and server MUST abort such handshakes. (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-6.4-2` | Clients and servers implementing this document SHOULD refuse SSL 3.0 handshakes. If they choose to support SSL 3.0, the resulting sessions MUST use the legacy master secret computation (§6.4) | MUST | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-4-1` | Clients and servers SHOULD NOT accept handshakes that do not use the extended master secret, especially if they rely on features like compound authentication that fall into the vulnerable cases described in Section 6.1. (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-3` | If the server receives a ClientHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy clients. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.2-5` | If a client receives a ServerHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy servers. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-1` | The client SHOULD NOT offer an abbreviated handshake to resume a session that does not use an extended master secret. (§5.3) | SHOULD NOT | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-2` | The client SHOULD NOT offer an abbreviated handshake to resume a session that does not use an extended master secret. Instead, it SHOULD offer a full handshake. (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-5` | If the original session did not use the "extended_master_secret" extension but the new ClientHello contains the extension, then the server MUST NOT perform the abbreviated handshake. Instead, it SHOULD continue with a full handshake (as described in Section 5.2) to negotiate a new session. (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.3-7` | If neither the original session nor the new ClientHello uses the extension, the server SHOULD abort the handshake. (§5.3) | SHOULD | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-6.4-1` | Clients and servers implementing this document SHOULD refuse SSL 3.0 handshakes. (§6.4) | SHOULD | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-6.2-1` | The session hashes of two different sessions need to be distinct; hence, the "Hash" function used to compute the "session_hash" needs to be collision resistant. As such, hash functions such as MD5 or SHA1 are NOT RECOMMENDED. (§6.2) | NOT RECOMMENDED | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7627-5.4-5` | If the original session uses an extended master secret but the ClientHello or ServerHello in the abbreviated handshake does not include the extension, it MAY be safe to continue the abbreviated handshake since it is protected by the extended master secret of the original session. (§5.4) | MAY | 5.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7627-5.1-1`](#rfc7627-5.1-1) If the client and server agree on this extension and a full handshake takes place, both client and server MUST use the extended master secret derivation algorithm, as defined in Section 4. (§5.1) | no test | no test carries this requirement id |
| [`RFC7627-5.2-1`](#rfc7627-5.2-1) In all handshakes, a client implementing this document MUST send the "extended_master_secret" extension in its ClientHello. (§5.2) | no test | no test carries this requirement id |
| [`RFC7627-5.2-2`](#rfc7627-5.2-2) If a server implementing this document receives the "extended_master_secret" extension, it MUST include the extension in its ServerHello message. (§5.2) | no test | no test carries this requirement id |
| [`RFC7627-5.2-4`](#rfc7627-5.2-4) If the server receives a ClientHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy clients. If it chooses to continue the handshake, then it MUST NOT include the extension in the ServerHello. (§5.2) | no test | no test carries this requirement id |
| [`RFC7627-5.2-6`](#rfc7627-5.2-6) If the client and server choose to continue a full handshake without the extension, they MUST use the standard master secret derivation for the new session. (§5.2) | no test | no test carries this requirement id |
| [`RFC7627-5.3-3`](#rfc7627-5.3-3) When offering an abbreviated handshake, the client MUST send the "extended_master_secret" extension in its ClientHello. (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-4`](#rfc7627-5.3-4) If the original session did not use the "extended_master_secret" extension but the new ClientHello contains the extension, then the server MUST NOT perform the abbreviated handshake. (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-6`](#rfc7627-5.3-6) If the original session used the "extended_master_secret" extension but the new ClientHello does not contain it, the server MUST abort the abbreviated handshake. (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-8`](#rfc7627-5.3-8) If the new ClientHello contains the extension and the server chooses to continue the handshake, then the server MUST include the "extended_master_secret" extension in its ServerHello message. (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-9`](#rfc7627-5.3-9) If the original session did not use the "extended_master_secret" extension but the new ServerHello contains the extension, the client MUST abort the handshake. (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.3-10`](#rfc7627-5.3-10) If the original session used the extension but the new ServerHello does not contain the extension, the client MUST abort the handshake. (§5.3) | no test | no test carries this requirement id |
| [`RFC7627-5.4-3`](#rfc7627-5.4-3) If a client or server chooses to continue an abbreviated handshake to resume a session that does not use the extended master secret, then the current connection becomes vulnerable to a man-in-the-middle handshake log synchronization attack as described in Section 1. Hence, the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication. (§5.4) | no test | no test carries this requirement id |
| [`RFC7627-5.4-4`](#rfc7627-5.4-4) If a client or server chooses to continue an abbreviated handshake to resume a session that does not use the extended master secret, then the current connection becomes vulnerable to a man-in-the-middle handshake log synchronization attack as described in Section 1. Hence, the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication. In particular, the client MUST disable renegotiation and any use of the "tls-unique" channel binding [RFC5929] on the current connection. (§5.4) | no test | no test carries this requirement id |
| [`RFC7627-5.4-6`](#rfc7627-5.4-6) If the original session uses an extended master secret but the ClientHello or ServerHello in the abbreviated handshake does not include the extension, it MAY be safe to continue the abbreviated handshake since it is protected by the extended master secret of the original session. This scenario may occur, for example, when a server that implements this extension establishes a session but the session is subsequently resumed at a different server that does not support the extension. Since such situations are unusual and likely to be the result of transient or inadvertent misconfigurations, this document recommends that the client and server MUST abort such handshakes. (§5.4) | no test | no test carries this requirement id |
| [`RFC7627-6.4-2`](#rfc7627-6.4-2) Clients and servers implementing this document SHOULD refuse SSL 3.0 handshakes. If they choose to support SSL 3.0, the resulting sessions MUST use the legacy master secret computation (§6.4) | no test | no test carries this requirement id |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7627-5.1-1`](#rfc7627-5.1-1)

If the client and server agree on this extension and a full handshake takes place, both client and server MUST use the extended master secret derivation algorithm, as defined in Section 4. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.1-1, so there is no linked test.

### [`RFC7627-5.2-1`](#rfc7627-5.2-1)

In all handshakes, a client implementing this document MUST send the "extended_master_secret" extension in its ClientHello. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.2-1, so there is no linked test.

### [`RFC7627-5.2-2`](#rfc7627-5.2-2)

If a server implementing this document receives the "extended_master_secret" extension, it MUST include the extension in its ServerHello message. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.2-2, so there is no linked test.

### [`RFC7627-5.2-4`](#rfc7627-5.2-4)

If the server receives a ClientHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy clients. If it chooses to continue the handshake, then it MUST NOT include the extension in the ServerHello. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.2-4, so there is no linked test.

### [`RFC7627-5.2-6`](#rfc7627-5.2-6)

If the client and server choose to continue a full handshake without the extension, they MUST use the standard master secret derivation for the new session. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.2-6, so there is no linked test.

### [`RFC7627-5.3-3`](#rfc7627-5.3-3)

When offering an abbreviated handshake, the client MUST send the "extended_master_secret" extension in its ClientHello. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.3-3, so there is no linked test.

### [`RFC7627-5.3-4`](#rfc7627-5.3-4)

If the original session did not use the "extended_master_secret" extension but the new ClientHello contains the extension, then the server MUST NOT perform the abbreviated handshake. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.3-4, so there is no linked test.

### [`RFC7627-5.3-6`](#rfc7627-5.3-6)

If the original session used the "extended_master_secret" extension but the new ClientHello does not contain it, the server MUST abort the abbreviated handshake. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.3-6, so there is no linked test.

### [`RFC7627-5.3-8`](#rfc7627-5.3-8)

If the new ClientHello contains the extension and the server chooses to continue the handshake, then the server MUST include the "extended_master_secret" extension in its ServerHello message. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.3-8, so there is no linked test.

### [`RFC7627-5.3-9`](#rfc7627-5.3-9)

If the original session did not use the "extended_master_secret" extension but the new ServerHello contains the extension, the client MUST abort the handshake. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.3-9, so there is no linked test.

### [`RFC7627-5.3-10`](#rfc7627-5.3-10)

If the original session used the extension but the new ServerHello does not contain the extension, the client MUST abort the handshake. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.3-10, so there is no linked test.

### [`RFC7627-5.4-1`](#rfc7627-5.4-1)

If a client or server chooses to continue with a full handshake without the extended master secret extension, then the new session becomes vulnerable to the man-in-the-middle key synchronization attack described in Section 1. Hence, the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication. (§5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`checkEAPTLS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L464) | interop/nightly | mutant, verified |
| positive | [`TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7627_eap_tls_export_refusal_test.go#L200) | unit/verify | mutant, verified |

### [`RFC7627-5.4-2`](#rfc7627-5.4-2)

If a client or server chooses to continue with a full handshake without the extended master secret extension, then the new session becomes vulnerable to the man-in-the-middle key synchronization attack described in Section 1. Hence, the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication. In particular, it MUST disable [RFC5705] and any Extensible Authentication Protocol (EAP) relying on compound authentication [COMPOUND-AUTH]. (§5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`checkEAPTLS`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/ipsec/checkers.go#L469) | interop/nightly | mutant, verified |
| positive | [`TestEAPTLSExportSucceedsOnTLS12WithExtendedMasterSecret`](https://github.com/ze-software/ze/blob/main/internal/core/eap/rfc7627_eap_tls_export_refusal_test.go#L203) | unit/verify | mutant, verified |

### [`RFC7627-5.4-3`](#rfc7627-5.4-3)

If a client or server chooses to continue an abbreviated handshake to resume a session that does not use the extended master secret, then the current connection becomes vulnerable to a man-in-the-middle handshake log synchronization attack as described in Section 1. Hence, the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication. (§5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.4-3, so there is no linked test.

### [`RFC7627-5.4-4`](#rfc7627-5.4-4)

If a client or server chooses to continue an abbreviated handshake to resume a session that does not use the extended master secret, then the current connection becomes vulnerable to a man-in-the-middle handshake log synchronization attack as described in Section 1. Hence, the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication. In particular, the client MUST disable renegotiation and any use of the "tls-unique" channel binding [RFC5929] on the current connection. (§5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.4-4, so there is no linked test.

### [`RFC7627-5.4-6`](#rfc7627-5.4-6)

If the original session uses an extended master secret but the ClientHello or ServerHello in the abbreviated handshake does not include the extension, it MAY be safe to continue the abbreviated handshake since it is protected by the extended master secret of the original session. This scenario may occur, for example, when a server that implements this extension establishes a session but the session is subsequently resumed at a different server that does not support the extension. Since such situations are unusual and likely to be the result of transient or inadvertent misconfigurations, this document recommends that the client and server MUST abort such handshakes. (§5.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-5.4-6, so there is no linked test.

### [`RFC7627-6.4-2`](#rfc7627-6.4-2)

Clients and servers implementing this document SHOULD refuse SSL 3.0 handshakes. If they choose to support SSL 3.0, the resulting sessions MUST use the legacy master secret computation (§6.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7627-6.4-2, so there is no linked test.

## Requirement list review

No requirement list review is recorded for RFC 7627, so there is no recorded check of its requirements against the original RFC, sentence by sentence.

## Superseded

RFC 7627 is obsoleted by RFC 9846.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC7627-5.1-1`](#rfc7627-5.1-1) If the client and server agree on this extension and a full handshake takes place, both client and server MUST use the extended master secret derivation algorithm, as defined in Section 4. (§5.1) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-1`](#rfc7627-5.2-1) In all handshakes, a client implementing this document MUST send the "extended_master_secret" extension in its ClientHello. (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-2`](#rfc7627-5.2-2) If a server implementing this document receives the "extended_master_secret" extension, it MUST include the extension in its ServerHello message. (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-4`](#rfc7627-5.2-4) If the server receives a ClientHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy clients. If it chooses to continue the handshake, then it MUST NOT include the extension in the ServerHello. (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-6`](#rfc7627-5.2-6) If the client and server choose to continue a full handshake without the extension, they MUST use the standard master secret derivation for the new session. (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-3`](#rfc7627-5.3-3) When offering an abbreviated handshake, the client MUST send the "extended_master_secret" extension in its ClientHello. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-4`](#rfc7627-5.3-4) If the original session did not use the "extended_master_secret" extension but the new ClientHello contains the extension, then the server MUST NOT perform the abbreviated handshake. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-6`](#rfc7627-5.3-6) If the original session used the "extended_master_secret" extension but the new ClientHello does not contain it, the server MUST abort the abbreviated handshake. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-8`](#rfc7627-5.3-8) If the new ClientHello contains the extension and the server chooses to continue the handshake, then the server MUST include the "extended_master_secret" extension in its ServerHello message. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-9`](#rfc7627-5.3-9) If the original session did not use the "extended_master_secret" extension but the new ServerHello contains the extension, the client MUST abort the handshake. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-10`](#rfc7627-5.3-10) If the original session used the extension but the new ServerHello does not contain the extension, the client MUST abort the handshake. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-1`](#rfc7627-5.4-1) If a client or server chooses to continue with a full handshake without the extended master secret extension, then the new session becomes vulnerable to the man-in-the-middle key synchronization attack described in Section 1. Hence, the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication. (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-2`](#rfc7627-5.4-2) If a client or server chooses to continue with a full handshake without the extended master secret extension, then the new session becomes vulnerable to the man-in-the-middle key synchronization attack described in Section 1. Hence, the client or server MUST NOT export any key material based on the new master secret for any subsequent application-level authentication. In particular, it MUST disable [RFC5705] and any Extensible Authentication Protocol (EAP) relying on compound authentication [COMPOUND-AUTH]. (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-3`](#rfc7627-5.4-3) If a client or server chooses to continue an abbreviated handshake to resume a session that does not use the extended master secret, then the current connection becomes vulnerable to a man-in-the-middle handshake log synchronization attack as described in Section 1. Hence, the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication. (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-4`](#rfc7627-5.4-4) If a client or server chooses to continue an abbreviated handshake to resume a session that does not use the extended master secret, then the current connection becomes vulnerable to a man-in-the-middle handshake log synchronization attack as described in Section 1. Hence, the client or server MUST NOT use the current handshake's "verify_data" for application-level authentication. In particular, the client MUST disable renegotiation and any use of the "tls-unique" channel binding [RFC5929] on the current connection. (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-6`](#rfc7627-5.4-6) If the original session uses an extended master secret but the ClientHello or ServerHello in the abbreviated handshake does not include the extension, it MAY be safe to continue the abbreviated handshake since it is protected by the extended master secret of the original session. This scenario may occur, for example, when a server that implements this extension establishes a session but the session is subsequently resumed at a different server that does not support the extension. Since such situations are unusual and likely to be the result of transient or inadvertent misconfigurations, this document recommends that the client and server MUST abort such handshakes. (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-6.4-2`](#rfc7627-6.4-2) Clients and servers implementing this document SHOULD refuse SSL 3.0 handshakes. If they choose to support SSL 3.0, the resulting sessions MUST use the legacy master secret computation (§6.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-4-1`](#rfc7627-4-1) Clients and servers SHOULD NOT accept handshakes that do not use the extended master secret, especially if they rely on features like compound authentication that fall into the vulnerable cases described in Section 6.1. (§4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-3`](#rfc7627-5.2-3) If the server receives a ClientHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy clients. (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.2-5`](#rfc7627-5.2-5) If a client receives a ServerHello without the extension, it SHOULD abort the handshake if it does not wish to interoperate with legacy servers. (§5.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-1`](#rfc7627-5.3-1) The client SHOULD NOT offer an abbreviated handshake to resume a session that does not use an extended master secret. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-2`](#rfc7627-5.3-2) The client SHOULD NOT offer an abbreviated handshake to resume a session that does not use an extended master secret. Instead, it SHOULD offer a full handshake. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-5`](#rfc7627-5.3-5) If the original session did not use the "extended_master_secret" extension but the new ClientHello contains the extension, then the server MUST NOT perform the abbreviated handshake. Instead, it SHOULD continue with a full handshake (as described in Section 5.2) to negotiate a new session. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.3-7`](#rfc7627-5.3-7) If neither the original session nor the new ClientHello uses the extension, the server SHOULD abort the handshake. (§5.3) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-6.4-1`](#rfc7627-6.4-1) Clients and servers implementing this document SHOULD refuse SSL 3.0 handshakes. (§6.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-6.2-1`](#rfc7627-6.2-1) The session hashes of two different sessions need to be distinct; hence, the "Hash" function used to compute the "session_hash" needs to be collision resistant. As such, hash functions such as MD5 or SHA1 are NOT RECOMMENDED. (§6.2) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
| [`RFC7627-5.4-5`](#rfc7627-5.4-5) If the original session uses an extended master secret but the ClientHello or ServerHello in the abbreviated handshake does not include the extension, it MAY be safe to continue the abbreviated handshake since it is protected by the extended master secret of the original session. (§5.4) | unresolved | not stated | RFC 9846 obsoletes this document and rfc/full/rfc9846.txt is not in this repository, so the successor requirement stating this obligation cannot be named or checked. The Meta note above records what is known about RFC 9846 without its text |
