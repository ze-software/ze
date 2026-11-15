# RFC 2867 - RADIUS Accounting Modifications for Tunnel Protocol Support

RADIUS tunnel accounting. Every requirement listed for RFC 2867, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 1 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 1 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 1 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 1 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 1 | of 1 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 1 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 1 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 1 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 1 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 1 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 1 | of 17 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (out-of-scope), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 1 mandatory requirements: they add up to 1. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | RADIUS tunnel accounting |
| Automatic checks | Excluded (out-of-scope) |
| Requirements | 17 |
| Mandatory requirements (MUST level), not checked | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 1 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc2867.md` |
| Requirement table | `rfc/requirements/rfc2867.md` |
| RFC text | `rfc/full/rfc2867.txt` |

## Automatic checks

Excluded from automatic checks (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the lns profile. The extraction is COMPLETE: the source text is at rfc/full/rfc2867.txt, every capitalised MUST-level sentence is a row of this summary or an exclusion in rfc/extraction/rfc2867.json (walked 2026-10-26), and none is gated. Ze's accounting records carry no tunnel attributes and no tunnel Acct-Status-Type values (internal/component/l2tp/plugins/authradius/acct.go buildAcctPacket).

## Published support status

**Area:** RADIUS tunnel accounting

**Proof:** 0 of 1 MUST proven

**Evidence:** -

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 1 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **1** | every mandatory requirement appears in exactly one group above |

**No test and no annotation (1):** [`RFC2867-5-1`](#rfc2867-5-1)

## Figures

- **MUST bar**: Not assessed 1
- **Overview bar**: Not assessed 17

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 1 |
| MUST rows kept (the MUST bar) | 1 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 1 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 16 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 16 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc2867.json](../../../data/rfc/rfc2867.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2867-5-1` | 0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-2-1` | When implementing RADIUS accounting for L2TP/PPTP tunneling, the Call-Serial-Number SHOULD be used in the Acct-Tunnel-Connection attribute. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-2-2` | In PPTP the combination of IP Address and Call-Serial-Number SHOULD be unique, but this is not required. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.1-2` | If this value is used, the following attributes SHOULD also be included in the Accounting-Request packet: User-Name (1) NAS-IP-Address (4) Acct-Delay-Time (41) Event-Timestamp (55) Tunnel-Type (64) Tunnel-Medium-Type (65) Tunnel-Client-Endpoint (66) Tunnel-Server-Endpoint (67) Acct-Tunnel-Connection (68) (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.2-2` | If this value is used, the following attributes SHOULD also be included in the Accounting-Request packet: User-Name (1) NAS-IP-Address (4) Acct-Delay-Time (41) Acct-Input-Octets (42) Acct-Output-Octets (43) Acct-Session-Id (44) Acct-Session-Time (46) Acct-Input-Packets (47) Acct-Output-Packets (48) Acct-Terminate-Cause (49) Acct-Multi-Session-Id (51) Event-Timestamp (55) Tunnel-Type (64) Tunnel-Medium-Type (65) Tunnel-Client-Endpoint (66) Tunnel-Server-Endpoint (67) Acct-Tunnel-Connection (68) Acct-Tunnel-Packets-Lost (86) (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.3-2` | If this value is used, the following attributes SHOULD also be included in the Accounting-Request packet: User-Name (1) NAS-IP-Address (4) Acct-Delay-Time (41) Acct-Terminate-Cause (49) Event-Timestamp (55) Tunnel-Type (64) Tunnel-Medium-Type (65) Tunnel-Client-Endpoint (66) Tunnel-Server-Endpoint (67) Acct-Tunnel-Connection (68) (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.4-2` | If this value is used, the following attributes SHOULD also be included in the Accounting-Request packet: User-Name (1) NAS-IP-Address (4) NAS-Port (5) Acct-Delay-Time (41) Event-Timestamp (55) Tunnel-Type (64) Tunnel-Medium-Type (65) Tunnel-Client-Endpoint (66) Tunnel-Server-Endpoint (67) Acct-Tunnel-Connection (68) (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.5-2` | If this value is used, the following attributes SHOULD also be included in the Accounting-Request packet: User-Name (1) NAS-IP-Address (4) NAS-Port (5) Acct-Delay-Time (41) Acct-Input-Octets (42) Acct-Output-Octets (43) Acct-Session-Id (44) Acct-Session-Time (46) Acct-Input-Packets (47) Acct-Output-Packets (48) Acct-Terminate-Cause (49) Acct-Multi-Session-Id (51) Event-Timestamp (55) NAS-Port-Type (61) Tunnel-Type (64) Tunnel-Medium-Type (65) Tunnel-Client-Endpoint (66) Tunnel-Server-Endpoint (67) Acct-Tunnel-Connection (68) Acct-Tunnel-Packets-Lost (86) (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.6-2` | If this value is used, the following attributes SHOULD also be included in the Accounting-Request packet: User-Name (1) NAS-IP-Address (4) Acct-Delay-Time (41) Acct-Terminate-Cause (49) Event-Timestamp (55) Tunnel-Type (64) Tunnel-Medium-Type (65) Tunnel-Client-Endpoint (66) Tunnel-Server-Endpoint (67) Acct-Tunnel-Connection (68) (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-4.1-1` | It SHOULD be included in Accounting-Request packets which contain an Acct-Status-Type attribute having the value Start, Stop or any of the values described above. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-4.2-1` | It SHOULD be included in Accounting-Request packets which contain an Acct-Status-Type attribute having the value Tunnel-Link-Stop. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.1-1` | Value 9 Description This value MAY be used to mark the establishment of a tunnel with another node. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.2-1` | Value 10 Description This value MAY be used to mark the destruction of a tunnel to or from another node. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.3-1` | Value 11 Description This value MAY be used to mark the rejection of the establishment of a tunnel with another node. (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.4-1` | Value 12 Description This value MAY be used to mark the creation of a tunnel link. (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.5-1` | Value 13 Description This value MAY be used to mark the destruction of a tunnel link. (§3.5) | MAY | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2867-3.6-1` | Value 14 Description This value MAY be used to mark the rejection of the establishment of a new link in an existing tunnel. (§3.6) | MAY | 3.6 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2867-5-1`](#rfc2867-5-1) 0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§5) | no test | no test carries this requirement id |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC2867-5-1`](#rfc2867-5-1)

0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2867-5-1, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc2867.txt |
| Source fingerprint | d0d090f6ce3be9a5 |
| Record | rfc/extraction/rfc2867.json |
| Mapped sentences | 1 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.2` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.3` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.4` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.5` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.6` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4.2` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `9` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `10` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

The review of RFC 2867 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 2867, so its obligations are stated where they were written.
