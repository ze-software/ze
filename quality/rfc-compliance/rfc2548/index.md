# RFC 2548 - Microsoft Vendor-specific RADIUS Attributes

Microsoft vendor-specific RADIUS attributes. Every requirement listed for RFC 2548, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 31 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 31 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 31 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 31 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 4 | 4 of 4 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 31 | of 31 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 31 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 31 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 31 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 31 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 31 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 31 | of 65 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (out-of-scope), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 31 mandatory requirements: they add up to 31. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | Microsoft vendor-specific RADIUS attributes |
| Automatic checks | Excluded (out-of-scope) |
| Requirements | 65 |
| Mandatory requirements (MUST level), not checked | 31 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 31 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tests linked to requirements | 4 |
| Recorded audit verdicts | 0 |
| Fault test records | 4 |
| Summary | `rfc/short/rfc2548.md` |
| Requirement table | `rfc/requirements/rfc2548.md` |
| RFC text | `rfc/full/rfc2548.txt` |

## Automatic checks

Excluded from automatic checks (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-11-08 (decision D-8 of spec-home-cpe-23b-dns-source-consumers): Thomas listed RFC 2548 in the optional tier of the lns profile, and the owner rule in ai/META-DECISIONS.md (Goal and scope) counts an optional RFC only once he brings it into scope. The extraction is COMPLETE: the source text is at rfc/full/rfc2548.txt and every capitalised MUST-level sentence is a row of this summary, mapped in rfc/extraction/rfc2548.json; none is gated.

## Published support status

**Area:** Microsoft vendor-specific RADIUS attributes

**Test verification:** 0 of 31 MUST proven

**Evidence:** -

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 31 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **31** | every mandatory requirement appears in exactly one group above |

**No test and no annotation (31):** [`RFC2548-2-1`](#rfc2548-2-1), [`RFC2548-2-2`](#rfc2548-2-2), [`RFC2548-2.1.3-1`](#rfc2548-2.1.3-1), [`RFC2548-2.1.6-1`](#rfc2548-2.1.6-1), [`RFC2548-2.1.7-1`](#rfc2548-2.1.7-1), [`RFC2548-2.1.7-2`](#rfc2548-2.1.7-2), [`RFC2548-2.1.7-3`](#rfc2548-2.1.7-3), [`RFC2548-2.1.8-1`](#rfc2548-2.1.8-1), [`RFC2548-2.2-1`](#rfc2548-2.2-1), [`RFC2548-2.3.2-1`](#rfc2548-2.3.2-1), [`RFC2548-2.3.2-2`](#rfc2548-2.3.2-2), [`RFC2548-2.3.3-1`](#rfc2548-2.3.3-1), [`RFC2548-2.3.4-1`](#rfc2548-2.3.4-1), [`RFC2548-2.3.4-2`](#rfc2548-2.3.4-2), [`RFC2548-2.4.1-1`](#rfc2548-2.4.1-1), [`RFC2548-2.4.2-1`](#rfc2548-2.4.2-1), [`RFC2548-2.4.2-2`](#rfc2548-2.4.2-2), [`RFC2548-2.4.2-3`](#rfc2548-2.4.2-3), [`RFC2548-2.4.2-4`](#rfc2548-2.4.2-4), [`RFC2548-2.4.2-5`](#rfc2548-2.4.2-5), [`RFC2548-2.4.2-6`](#rfc2548-2.4.2-6), [`RFC2548-2.4.3-1`](#rfc2548-2.4.3-1), [`RFC2548-2.4.3-2`](#rfc2548-2.4.3-2), [`RFC2548-2.4.3-3`](#rfc2548-2.4.3-3), [`RFC2548-2.4.3-4`](#rfc2548-2.4.3-4), [`RFC2548-2.4.3-5`](#rfc2548-2.4.3-5), [`RFC2548-2.4.3-6`](#rfc2548-2.4.3-6), [`RFC2548-2.4.4-1`](#rfc2548-2.4.4-1), [`RFC2548-2.7.3-1`](#rfc2548-2.7.3-1), [`RFC2548-2.7.3-2`](#rfc2548-2.7.3-2), [`RFC2548-3-1`](#rfc2548-3-1)

## Figures

- **MUST bar**: Not assessed 31
- **Overview bar**: SHOULD verified 2, Not assessed 63

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 31 |
| MUST rows kept (the MUST bar) | 31 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 31 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 34 |
| SHOULD verified | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 32 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc2548.json](../../../data/rfc/rfc2548.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2548-2-1` | The Length field of the Vendor-Specific Attribute MUST be set equal to the sum of the Vendor-Length fields of the sub-attributes contained in the Vendor-Specific Attribute, plus six. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2-2` | The Vendor-ID field of the Vendor-Specific Attribute(s) MUST be set to decimal 311 (Microsoft). (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.3-1` | If it is zero, the NT-Response field MUST be ignored and the LM-Response field used. (§2.1.3) | MUST | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.6-1` | Otherwise, the LM-New-Password and LM-Old-Password fields MUST be used. (§2.1.6) | MUST | 2.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.7-1` | The value of this field MUST be identical to that in the Ident field in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT- Enc-PW and MS-CHAP-PW-2 attributes contained in a single Access- Request packet. (§2.1.7) | MUST | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.7-2` | If least significant bit of the field is zero, the NT-Response field MUST be ignored and the LM- Response field used instead. (§2.1.7) | MUST | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.7-3` | If this bit is set, at least one instance of the MS-CHAP-LM-Enc-PW attribute MUST be included in the packet. (§2.1.7) | MUST | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.8-1` | The value of this field MUST be identical in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT-Enc-PW and MS- CHAP-PW-2 attributes which are present in the same Access-Request packet. (§2.1.8) | MUST | 2.1.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.2-1` | The value of this field MUST be identical in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT-Enc-PW and MS- CHAP-PW-2 attributes which are present in the same Access-Request packet. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.3.2-1` | It is reserved for future use and MUST be zero. (§2.3.2) | MUST | 2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.3.2-2` | Reserved This field is reserved for future use and MUST be zero. (§2.3.2) | MUST | 2.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.3.3-1` | This string MUST be included in the Message field of the MS-CHAP- V2 Success packet sent from the NAS to the peer. (§2.3.3) | MUST | 2.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.3.4-1` | The value of this field MUST be identical to that in the Ident field in all instances of the MS-CHAP-NT-Enc-PW contained in the Access-Request packet. (§2.3.4) | MUST | 2.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.3.4-2` | This field is reserved for future use and MUST be zero. (§2.3.4) | MUST | 2.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.1-1` | The Keys field MUST be encrypted by the RADIUS server using the same method defined for the User-Password Attribute [3]. (§2.4.1) | MUST | 2.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.2-1` | The most significant bit (leftmost) of the Salt field MUST be set (1). (§2.4.2) | MUST | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.2-2` | The contents of each Salt field in a given Access-Accept packet MUST be unique. (§2.4.2) | MUST | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.2-3` | If the combined length (in octets) of the unencrypted Key-Length and Key sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§2.4.2) | MUST | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.2-4` | The String field MUST be encrypted as follows, prior to transmission: (§2.4.2) | MUST | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.2-5` | When the RADIUS server receives the attribute from the external server, it MUST correctly set the Salt field and encrypt the String field before transmitting it to the RADIUS client. (§2.4.2) | MUST | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.2-6` | If the channel used to communicate the MS-MPPE-Send-Key attribute is not secure from eavesdropping, the attribute MUST be cryptographically protected. (§2.4.2) | MUST | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.3-1` | The most significant bit (leftmost) of the Salt field MUST be set (1). (§2.4.3) | MUST | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.3-2` | The contents of each Salt field in a given Access-Accept packet MUST be unique. (§2.4.3) | MUST | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.3-3` | If the combined length (in octets) of the unencrypted Key-Length and Key sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§2.4.3) | MUST | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.3-4` | The String field MUST be encrypted as follows, prior to transmission: (§2.4.3) | MUST | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.3-5` | When the RADIUS server receives the attribute from the external server, it MUST correctly set the Salt field and encrypt the String field before transmitting it to the RADIUS client. (§2.4.3) | MUST | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.3-6` | If the channel used to communicate the MS-MPPE-Recv-Key attribute is not secure from eavesdropping, the attribute MUST be cryptographically protected. (§2.4.3) | MUST | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.4-1` | If the Policy field is equal to 2 (Encryption-Required), any of the encryption types specified in the MS-MPPE-Encryption-Types Attribute MAY be used, but at least one MUST be used. (§2.4.4) | MUST | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.3-1` | If multiple MS-Filter Attributes are contained within a packet, they MUST be in order and they MUST be consecutive attributes in the packet. (§2.7.3) | MUST | 2.7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.3-2` | If multiple MS-Filter Attributes occur in a single Access-Accept packet, the Filter field from each MUST be concatenated in the order received to form the actual filter. (§2.7.3) | MUST | 2.7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-3-1` | 0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2-3` | if this is done, the sub-attributes SHOULD be packed as a sequence of Vendor-Type/Vendor-Length/Value triples following the inital Type, Length and Vendor-ID fields. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.3-2` | The LM-Response field MAY still be used (if non-empty), but the NT-Response SHOULD be tried first. (§2.1.3) | SHOULD | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.3-3` | The LM-Response field is 24 octets in length and holds an encoded function of the password and the received challenge. If this field is empty, it SHOULD be zero-filled. (§2.1.3) | SHOULD | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.3-4` | The NT-Response field is 24 octets in length and holds an encoded function of the password and the received challenge. If this field is empty, it SHOULD be zero-filled. (§2.1.3) | SHOULD | 2.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.6-2` | and NT-Old-Password fields are valid and SHOULD be used. (§2.1.6) | SHOULD | 2.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.7-4` | The LM-Response field is 24 octets in length and holds an encoded function of the password and the received challenge. If this field is empty, it SHOULD be zero-filled. (§2.1.7) | SHOULD | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.7-5` | The NT-Response field is 24 octets in length and holds an encoded function of the password and the received challenge. If this field is empty, it SHOULD be zero-filled. (§2.1.7) | SHOULD | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.7-6` | The LM-Response field MAY still be used (if present), but the NT- Response SHOULD be tried first. (§2.1.7) | SHOULD | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.7-7` | If bit 1 of the Flags field is one, the Old-LM-Hash field is valid and SHOULD be used. (§2.1.7) | SHOULD | 2.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.2-7` | The external server SHOULD, however, return the attribute as defined above; the Salt field SHOULD be zero-filled and padding of the String field SHOULD be done. (§2.4.2) | SHOULD | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.3-7` | The external server SHOULD, however, return the attribute as defined above; the Salt field SHOULD be zero-filled and padding of the String field SHOULD be done. (§2.4.3) | SHOULD | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.2-1` | This attribute SHOULD be included in packets containing an MS-RAS-Vendor Attribute; it SHOULD NOT be sent in packets which do not contain an MS-RAS-Vendor Attribute. (§2.7.2) | SHOULD | 2.7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.2-2` | The actual format of the information is vendor specific, and a robust implementation SHOULD support the field as undistinguished octets. (§2.7.2) | SHOULD | 2.7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-4-1` | Therefore, these attributes SHOULD NOT be used on networks where untrusted RADIUS proxies reside. (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2-4` | More than one sub-attribute MAY be transmitted in a single Vendor-Specific Attribute; (§2) | MAY | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.2-1` | This Attribute contains the challenge sent by a NAS to a Microsoft Challenge-Handshake Authentication Protocol (MS-CHAP) user. It MAY be used in both Access-Request and Access-Challenge packets. (§2.1.2) | MAY | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.1.4-1` | The MS-CHAP-Domain Attribute indicates the Windows NT domain in which the user was authenticated. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.1.4) | MAY | 2.1.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.2-8` | This attribute MAY be used to pass a key from an external (e.g., EAP [15]) server to the RADIUS server. (§2.4.2) | MAY | 2.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.3-8` | This attribute MAY be used to pass a key from an external (e.g., EAP [15]) server to the RADIUS server. (§2.4.3) | MAY | 2.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.4.4-2` | If the Policy field is equal to 1 (Encryption-Allowed), any or none of the encryption types specified in the MS-MPPE-Encryption-Types Attribute MAY be used. (§2.4.4) | MAY | 2.4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.5.1-1` | This Attribute describes whether the use of BAP is allowed, disallowed or required on new multilink calls. It MAY be used in Access-Accept packets. (§2.5.1) | MAY | 2.5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.5.3-1` | The MS-Link-Drop-Time-Limit Attribute indicates the length of time (in seconds) that a link must be underutilized before it is dropped. It MAY only be included in Access-Accept packets. (§2.5.3) | MAY | 2.5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.6.1-1` | The MS-Old-ARAP-Password Attribute is used to transmit the old ARAP password during an ARAP password change operation. It MAY be included in Access-Request packets. (§2.6.1) | MAY | 2.6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.6.2-1` | The MS-New-ARAP-Password Attribute is used to transmit the new ARAP password during an ARAP password change operation. It MAY be included in Access-Request packets. (§2.6.2) | MAY | 2.6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.6.3-1` | The MS-ARAP-Password-Change-Reason Attribute is used to indicate reason for a server-initiated password change. It MAY be included in Access-Challenge packets. (§2.6.3) | MAY | 2.6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.1-1` | The MS-RAS-Vendor Attribute is used to indicate the manufacturer of the RADIUS client machine. It MAY be included in both Access- Request and Accounting-Request packets. (§2.7.1) | MAY | 2.7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.2-3` | It MAY be included in both Access-Request and Accounting-Request packets. (§2.7.2) | MAY | 2.7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.3-3` | The MS-Filter Attribute is used to transmit traffic filters. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.3) | MAY | 2.7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.4-1` | The MS-Acct-Auth-Type Attribute is used to represent the method used to authenticate the dial-up user. It MAY be included in Accounting-Request packets. (§2.7.4) | MAY | 2.7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.5-1` | The MS-Acct-EAP-Type Attribute is used to represent the Extensible Authentication Protocol (EAP) [15] type used to authenticate the dial-up user. It MAY be included in Accounting-Request packets. (§2.7.5) | MAY | 2.7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.6-1` | The MS-Primary-DNS-Server Attribute is used to indicate the address of the primary Domain Name Server (DNS) [16, 17] server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.6) | MAY | 2.7.6 | **positive:** `unit/verify` [`TestExtractMSDNSServers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2548_extract_dns_test.go#L15). **positive:** `unit/verify` [`TestPoolIPCPDNSFromRadius`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/pool/rfc2548_dns_source_test.go#L54). **negative:** no negative test |
| `RFC2548-2.7.7-1` | The MS-Secondary-DNS-Server Attribute is used to indicate the address of the secondary DNS server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.7) | MAY | 2.7.7 | **positive:** `unit/verify` [`TestExtractMSDNSServers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2548_extract_dns_test.go#L19). **positive:** `unit/verify` [`TestPoolIPCPDNSFromRadius`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/pool/rfc2548_dns_source_test.go#L58). **negative:** no negative test |
| `RFC2548-2.7.8-1` | The MS-Primary-NBNS-Server Attribute is used to indicate the address of the primary NetBIOS Name Server (NBNS) [18] server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.8) | MAY | 2.7.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2548-2.7.9-1` | The MS-Secondary-NBNS-Server Attribute is used to indicate the address of the secondary DNS server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.9) | MAY | 2.7.9 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2548-2-1`](#rfc2548-2-1) The Length field of the Vendor-Specific Attribute MUST be set equal to the sum of the Vendor-Length fields of the sub-attributes contained in the Vendor-Specific Attribute, plus six. (§2) | no test | no test carries this requirement id |
| [`RFC2548-2-2`](#rfc2548-2-2) The Vendor-ID field of the Vendor-Specific Attribute(s) MUST be set to decimal 311 (Microsoft). (§2) | no test | no test carries this requirement id |
| [`RFC2548-2.1.3-1`](#rfc2548-2.1.3-1) If it is zero, the NT-Response field MUST be ignored and the LM-Response field used. (§2.1.3) | no test | no test carries this requirement id |
| [`RFC2548-2.1.6-1`](#rfc2548-2.1.6-1) Otherwise, the LM-New-Password and LM-Old-Password fields MUST be used. (§2.1.6) | no test | no test carries this requirement id |
| [`RFC2548-2.1.7-1`](#rfc2548-2.1.7-1) The value of this field MUST be identical to that in the Ident field in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT- Enc-PW and MS-CHAP-PW-2 attributes contained in a single Access- Request packet. (§2.1.7) | no test | no test carries this requirement id |
| [`RFC2548-2.1.7-2`](#rfc2548-2.1.7-2) If least significant bit of the field is zero, the NT-Response field MUST be ignored and the LM- Response field used instead. (§2.1.7) | no test | no test carries this requirement id |
| [`RFC2548-2.1.7-3`](#rfc2548-2.1.7-3) If this bit is set, at least one instance of the MS-CHAP-LM-Enc-PW attribute MUST be included in the packet. (§2.1.7) | no test | no test carries this requirement id |
| [`RFC2548-2.1.8-1`](#rfc2548-2.1.8-1) The value of this field MUST be identical in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT-Enc-PW and MS- CHAP-PW-2 attributes which are present in the same Access-Request packet. (§2.1.8) | no test | no test carries this requirement id |
| [`RFC2548-2.2-1`](#rfc2548-2.2-1) The value of this field MUST be identical in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT-Enc-PW and MS- CHAP-PW-2 attributes which are present in the same Access-Request packet. (§2.2) | no test | no test carries this requirement id |
| [`RFC2548-2.3.2-1`](#rfc2548-2.3.2-1) It is reserved for future use and MUST be zero. (§2.3.2) | no test | no test carries this requirement id |
| [`RFC2548-2.3.2-2`](#rfc2548-2.3.2-2) Reserved This field is reserved for future use and MUST be zero. (§2.3.2) | no test | no test carries this requirement id |
| [`RFC2548-2.3.3-1`](#rfc2548-2.3.3-1) This string MUST be included in the Message field of the MS-CHAP- V2 Success packet sent from the NAS to the peer. (§2.3.3) | no test | no test carries this requirement id |
| [`RFC2548-2.3.4-1`](#rfc2548-2.3.4-1) The value of this field MUST be identical to that in the Ident field in all instances of the MS-CHAP-NT-Enc-PW contained in the Access-Request packet. (§2.3.4) | no test | no test carries this requirement id |
| [`RFC2548-2.3.4-2`](#rfc2548-2.3.4-2) This field is reserved for future use and MUST be zero. (§2.3.4) | no test | no test carries this requirement id |
| [`RFC2548-2.4.1-1`](#rfc2548-2.4.1-1) The Keys field MUST be encrypted by the RADIUS server using the same method defined for the User-Password Attribute [3]. (§2.4.1) | no test | no test carries this requirement id |
| [`RFC2548-2.4.2-1`](#rfc2548-2.4.2-1) The most significant bit (leftmost) of the Salt field MUST be set (1). (§2.4.2) | no test | no test carries this requirement id |
| [`RFC2548-2.4.2-2`](#rfc2548-2.4.2-2) The contents of each Salt field in a given Access-Accept packet MUST be unique. (§2.4.2) | no test | no test carries this requirement id |
| [`RFC2548-2.4.2-3`](#rfc2548-2.4.2-3) If the combined length (in octets) of the unencrypted Key-Length and Key sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§2.4.2) | no test | no test carries this requirement id |
| [`RFC2548-2.4.2-4`](#rfc2548-2.4.2-4) The String field MUST be encrypted as follows, prior to transmission: (§2.4.2) | no test | no test carries this requirement id |
| [`RFC2548-2.4.2-5`](#rfc2548-2.4.2-5) When the RADIUS server receives the attribute from the external server, it MUST correctly set the Salt field and encrypt the String field before transmitting it to the RADIUS client. (§2.4.2) | no test | no test carries this requirement id |
| [`RFC2548-2.4.2-6`](#rfc2548-2.4.2-6) If the channel used to communicate the MS-MPPE-Send-Key attribute is not secure from eavesdropping, the attribute MUST be cryptographically protected. (§2.4.2) | no test | no test carries this requirement id |
| [`RFC2548-2.4.3-1`](#rfc2548-2.4.3-1) The most significant bit (leftmost) of the Salt field MUST be set (1). (§2.4.3) | no test | no test carries this requirement id |
| [`RFC2548-2.4.3-2`](#rfc2548-2.4.3-2) The contents of each Salt field in a given Access-Accept packet MUST be unique. (§2.4.3) | no test | no test carries this requirement id |
| [`RFC2548-2.4.3-3`](#rfc2548-2.4.3-3) If the combined length (in octets) of the unencrypted Key-Length and Key sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§2.4.3) | no test | no test carries this requirement id |
| [`RFC2548-2.4.3-4`](#rfc2548-2.4.3-4) The String field MUST be encrypted as follows, prior to transmission: (§2.4.3) | no test | no test carries this requirement id |
| [`RFC2548-2.4.3-5`](#rfc2548-2.4.3-5) When the RADIUS server receives the attribute from the external server, it MUST correctly set the Salt field and encrypt the String field before transmitting it to the RADIUS client. (§2.4.3) | no test | no test carries this requirement id |
| [`RFC2548-2.4.3-6`](#rfc2548-2.4.3-6) If the channel used to communicate the MS-MPPE-Recv-Key attribute is not secure from eavesdropping, the attribute MUST be cryptographically protected. (§2.4.3) | no test | no test carries this requirement id |
| [`RFC2548-2.4.4-1`](#rfc2548-2.4.4-1) If the Policy field is equal to 2 (Encryption-Required), any of the encryption types specified in the MS-MPPE-Encryption-Types Attribute MAY be used, but at least one MUST be used. (§2.4.4) | no test | no test carries this requirement id |
| [`RFC2548-2.7.3-1`](#rfc2548-2.7.3-1) If multiple MS-Filter Attributes are contained within a packet, they MUST be in order and they MUST be consecutive attributes in the packet. (§2.7.3) | no test | no test carries this requirement id |
| [`RFC2548-2.7.3-2`](#rfc2548-2.7.3-2) If multiple MS-Filter Attributes occur in a single Access-Accept packet, the Filter field from each MUST be concatenated in the order received to form the actual filter. (§2.7.3) | no test | no test carries this requirement id |
| [`RFC2548-3-1`](#rfc2548-3-1) 0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§3) | no test | no test carries this requirement id |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC2548-2-1`](#rfc2548-2-1)

The Length field of the Vendor-Specific Attribute MUST be set equal to the sum of the Vendor-Length fields of the sub-attributes contained in the Vendor-Specific Attribute, plus six. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2-1, so there is no linked test.

### [`RFC2548-2-2`](#rfc2548-2-2)

The Vendor-ID field of the Vendor-Specific Attribute(s) MUST be set to decimal 311 (Microsoft). (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2-2, so there is no linked test.

### [`RFC2548-2.1.3-1`](#rfc2548-2.1.3-1)

If it is zero, the NT-Response field MUST be ignored and the LM-Response field used. (§2.1.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.1.3-1, so there is no linked test.

### [`RFC2548-2.1.6-1`](#rfc2548-2.1.6-1)

Otherwise, the LM-New-Password and LM-Old-Password fields MUST be used. (§2.1.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.1.6-1, so there is no linked test.

### [`RFC2548-2.1.7-1`](#rfc2548-2.1.7-1)

The value of this field MUST be identical to that in the Ident field in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT- Enc-PW and MS-CHAP-PW-2 attributes contained in a single Access- Request packet. (§2.1.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.1.7-1, so there is no linked test.

### [`RFC2548-2.1.7-2`](#rfc2548-2.1.7-2)

If least significant bit of the field is zero, the NT-Response field MUST be ignored and the LM- Response field used instead. (§2.1.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.1.7-2, so there is no linked test.

### [`RFC2548-2.1.7-3`](#rfc2548-2.1.7-3)

If this bit is set, at least one instance of the MS-CHAP-LM-Enc-PW attribute MUST be included in the packet. (§2.1.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.1.7-3, so there is no linked test.

### [`RFC2548-2.1.8-1`](#rfc2548-2.1.8-1)

The value of this field MUST be identical in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT-Enc-PW and MS- CHAP-PW-2 attributes which are present in the same Access-Request packet. (§2.1.8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.1.8-1, so there is no linked test.

### [`RFC2548-2.2-1`](#rfc2548-2.2-1)

The value of this field MUST be identical in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT-Enc-PW and MS- CHAP-PW-2 attributes which are present in the same Access-Request packet. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.2-1, so there is no linked test.

### [`RFC2548-2.3.2-1`](#rfc2548-2.3.2-1)

It is reserved for future use and MUST be zero. (§2.3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.3.2-1, so there is no linked test.

### [`RFC2548-2.3.2-2`](#rfc2548-2.3.2-2)

Reserved This field is reserved for future use and MUST be zero. (§2.3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.3.2-2, so there is no linked test.

### [`RFC2548-2.3.3-1`](#rfc2548-2.3.3-1)

This string MUST be included in the Message field of the MS-CHAP- V2 Success packet sent from the NAS to the peer. (§2.3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.3.3-1, so there is no linked test.

### [`RFC2548-2.3.4-1`](#rfc2548-2.3.4-1)

The value of this field MUST be identical to that in the Ident field in all instances of the MS-CHAP-NT-Enc-PW contained in the Access-Request packet. (§2.3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.3.4-1, so there is no linked test.

### [`RFC2548-2.3.4-2`](#rfc2548-2.3.4-2)

This field is reserved for future use and MUST be zero. (§2.3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.3.4-2, so there is no linked test.

### [`RFC2548-2.4.1-1`](#rfc2548-2.4.1-1)

The Keys field MUST be encrypted by the RADIUS server using the same method defined for the User-Password Attribute [3]. (§2.4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.1-1, so there is no linked test.

### [`RFC2548-2.4.2-1`](#rfc2548-2.4.2-1)

The most significant bit (leftmost) of the Salt field MUST be set (1). (§2.4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.2-1, so there is no linked test.

### [`RFC2548-2.4.2-2`](#rfc2548-2.4.2-2)

The contents of each Salt field in a given Access-Accept packet MUST be unique. (§2.4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.2-2, so there is no linked test.

### [`RFC2548-2.4.2-3`](#rfc2548-2.4.2-3)

If the combined length (in octets) of the unencrypted Key-Length and Key sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§2.4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.2-3, so there is no linked test.

### [`RFC2548-2.4.2-4`](#rfc2548-2.4.2-4)

The String field MUST be encrypted as follows, prior to transmission: (§2.4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.2-4, so there is no linked test.

### [`RFC2548-2.4.2-5`](#rfc2548-2.4.2-5)

When the RADIUS server receives the attribute from the external server, it MUST correctly set the Salt field and encrypt the String field before transmitting it to the RADIUS client. (§2.4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.2-5, so there is no linked test.

### [`RFC2548-2.4.2-6`](#rfc2548-2.4.2-6)

If the channel used to communicate the MS-MPPE-Send-Key attribute is not secure from eavesdropping, the attribute MUST be cryptographically protected. (§2.4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.2-6, so there is no linked test.

### [`RFC2548-2.4.3-1`](#rfc2548-2.4.3-1)

The most significant bit (leftmost) of the Salt field MUST be set (1). (§2.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.3-1, so there is no linked test.

### [`RFC2548-2.4.3-2`](#rfc2548-2.4.3-2)

The contents of each Salt field in a given Access-Accept packet MUST be unique. (§2.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.3-2, so there is no linked test.

### [`RFC2548-2.4.3-3`](#rfc2548-2.4.3-3)

If the combined length (in octets) of the unencrypted Key-Length and Key sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§2.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.3-3, so there is no linked test.

### [`RFC2548-2.4.3-4`](#rfc2548-2.4.3-4)

The String field MUST be encrypted as follows, prior to transmission: (§2.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.3-4, so there is no linked test.

### [`RFC2548-2.4.3-5`](#rfc2548-2.4.3-5)

When the RADIUS server receives the attribute from the external server, it MUST correctly set the Salt field and encrypt the String field before transmitting it to the RADIUS client. (§2.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.3-5, so there is no linked test.

### [`RFC2548-2.4.3-6`](#rfc2548-2.4.3-6)

If the channel used to communicate the MS-MPPE-Recv-Key attribute is not secure from eavesdropping, the attribute MUST be cryptographically protected. (§2.4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.3-6, so there is no linked test.

### [`RFC2548-2.4.4-1`](#rfc2548-2.4.4-1)

If the Policy field is equal to 2 (Encryption-Required), any of the encryption types specified in the MS-MPPE-Encryption-Types Attribute MAY be used, but at least one MUST be used. (§2.4.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.4.4-1, so there is no linked test.

### [`RFC2548-2.7.3-1`](#rfc2548-2.7.3-1)

If multiple MS-Filter Attributes are contained within a packet, they MUST be in order and they MUST be consecutive attributes in the packet. (§2.7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.7.3-1, so there is no linked test.

### [`RFC2548-2.7.3-2`](#rfc2548-2.7.3-2)

If multiple MS-Filter Attributes occur in a single Access-Accept packet, the Filter field from each MUST be concatenated in the order received to form the actual filter. (§2.7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-2.7.3-2, so there is no linked test.

### [`RFC2548-3-1`](#rfc2548-3-1)

0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2548-3-1, so there is no linked test.

### [`RFC2548-2.7.6-1`](#rfc2548-2.7.6-1)

The MS-Primary-DNS-Server Attribute is used to indicate the address of the primary Domain Name Server (DNS) [16, 17] server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestExtractMSDNSServers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2548_extract_dns_test.go#L15) | unit/verify | mutant, verified |
| positive | [`TestPoolIPCPDNSFromRadius`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/pool/rfc2548_dns_source_test.go#L54) | unit/verify | mutant, verified |

### [`RFC2548-2.7.7-1`](#rfc2548-2.7.7-1)

The MS-Secondary-DNS-Server Attribute is used to indicate the address of the secondary DNS server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestExtractMSDNSServers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2548_extract_dns_test.go#L19) | unit/verify | mutant, verified |
| positive | [`TestPoolIPCPDNSFromRadius`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/pool/rfc2548_dns_source_test.go#L58) | unit/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-11-09 |
| Register | rfc2119 |
| Source | rfc/full/rfc2548.txt |
| Source fingerprint | d24114b74cccc66f |
| Record | rfc/extraction/rfc2548.json |
| Mapped sentences | 31 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2` | not stated | 2 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.2` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.3` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.4` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.5` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.6` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.7` | not stated | 3 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.8` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.2` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3.2` | not stated | 2 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3.3` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3.4` | not stated | 2 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.1` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.2` | not stated | 6 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.3` | not stated | 6 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.4` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.5` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.5` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.5.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.5.2` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.5.3` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6.2` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6.3` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6.4` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.2` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.3` | not stated | 2 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.4` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.5` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.6` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.7` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.8` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.9` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `10` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

The review of RFC 2548 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 2548, so its obligations are stated where they were written.
