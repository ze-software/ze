# RFC 2548 - Microsoft Vendor-specific RADIUS Attributes

Microsoft vendor-specific RADIUS attributes. Every requirement this repository extracted from RFC 2548, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 31 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 31 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 31 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 31 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 4 | 4 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 31 | of 65 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every count below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 31 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 31 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 31 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 31 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 31 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 31 | of 31 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 31 gated MUSTs: they add up to 31. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | Microsoft vendor-specific RADIUS attributes |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 65 |
| Gated MUST-level | 31 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 31 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 4 |
| Summary | `rfc/short/rfc2548.md` |
| Requirement shard | `rfc/requirements/rfc2548.md` |
| RFC text | `rfc/full/rfc2548.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-11-08 (decision D-8 of spec-home-cpe-23b-dns-source-consumers): Thomas listed RFC 2548 in the optional tier of the lns profile, and the owner rule in ai/META-DECISIONS.md (Goal and scope) counts an optional RFC only once he brings it into scope. The extraction is COMPLETE: the source text is at rfc/full/rfc2548.txt and every capitalised MUST-level sentence is a row of this summary, mapped in rfc/extraction/rfc2548.json; none is gated.

## What the public ledger says

**Area:** Microsoft vendor-specific RADIUS attributes

**Proof:** 0 of 31 MUST proven

**Evidence:** -

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 31 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **31** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (31):** [`RFC2548-2-1`](#rfc2548-2-1), [`RFC2548-2-2`](#rfc2548-2-2), [`RFC2548-2.1.3-1`](#rfc2548-2.1.3-1), [`RFC2548-2.1.6-1`](#rfc2548-2.1.6-1), [`RFC2548-2.1.7-1`](#rfc2548-2.1.7-1), [`RFC2548-2.1.7-2`](#rfc2548-2.1.7-2), [`RFC2548-2.1.7-3`](#rfc2548-2.1.7-3), [`RFC2548-2.1.8-1`](#rfc2548-2.1.8-1), [`RFC2548-2.2-1`](#rfc2548-2.2-1), [`RFC2548-2.3.2-1`](#rfc2548-2.3.2-1), [`RFC2548-2.3.2-2`](#rfc2548-2.3.2-2), [`RFC2548-2.3.3-1`](#rfc2548-2.3.3-1), [`RFC2548-2.3.4-1`](#rfc2548-2.3.4-1), [`RFC2548-2.3.4-2`](#rfc2548-2.3.4-2), [`RFC2548-2.4.1-1`](#rfc2548-2.4.1-1), [`RFC2548-2.4.2-1`](#rfc2548-2.4.2-1), [`RFC2548-2.4.2-2`](#rfc2548-2.4.2-2), [`RFC2548-2.4.2-3`](#rfc2548-2.4.2-3), [`RFC2548-2.4.2-4`](#rfc2548-2.4.2-4), [`RFC2548-2.4.2-5`](#rfc2548-2.4.2-5), [`RFC2548-2.4.2-6`](#rfc2548-2.4.2-6), [`RFC2548-2.4.3-1`](#rfc2548-2.4.3-1), [`RFC2548-2.4.3-2`](#rfc2548-2.4.3-2), [`RFC2548-2.4.3-3`](#rfc2548-2.4.3-3), [`RFC2548-2.4.3-4`](#rfc2548-2.4.3-4), [`RFC2548-2.4.3-5`](#rfc2548-2.4.3-5), [`RFC2548-2.4.3-6`](#rfc2548-2.4.3-6), [`RFC2548-2.4.4-1`](#rfc2548-2.4.4-1), [`RFC2548-2.7.3-1`](#rfc2548-2.7.3-1), [`RFC2548-2.7.3-2`](#rfc2548-2.7.3-2), [`RFC2548-3-1`](#rfc2548-3-1)

## Figures

- **MUST bar**: Ze correctness gap 31
- **Overview bar**: SHOULD proven 2, SHOULD not decided 32, Ze correctness gap 31

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 31 |
| MUST rows kept (the MUST bar) | 31 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 31 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 34 |
| SHOULD proven | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 32 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

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
| `RFC2548-2.7.6-1` | The MS-Primary-DNS-Server Attribute is used to indicate the address of the primary Domain Name Server (DNS) [16, 17] server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.6) | MAY | 2.7.6 | **positive:** `unit/verify` [`TestExtractMSDNSServers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2548_extract_dns_test.go#L15). **positive:** `unit/verify` [`TestPoolIPCPDNSFromRadius`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/pool/rfc2548_dns_source_test.go#L51). **negative:** no negative test |
| `RFC2548-2.7.7-1` | The MS-Secondary-DNS-Server Attribute is used to indicate the address of the secondary DNS server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.7) | MAY | 2.7.7 | **positive:** `unit/verify` [`TestExtractMSDNSServers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2548_extract_dns_test.go#L19). **positive:** `unit/verify` [`TestPoolIPCPDNSFromRadius`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/pool/rfc2548_dns_source_test.go#L55). **negative:** no negative test |
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

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2548-2-1`](#rfc2548-2-1)

The Length field of the Vendor-Specific Attribute MUST be set equal to the sum of the Vendor-Length fields of the sub-attributes contained in the Vendor-Specific Attribute, plus six. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2-1, so no unit is bound to it.

### [`RFC2548-2-2`](#rfc2548-2-2)

The Vendor-ID field of the Vendor-Specific Attribute(s) MUST be set to decimal 311 (Microsoft). (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2-2, so no unit is bound to it.

### [`RFC2548-2.1.3-1`](#rfc2548-2.1.3-1)

If it is zero, the NT-Response field MUST be ignored and the LM-Response field used. (§2.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.1.3-1, so no unit is bound to it.

### [`RFC2548-2.1.6-1`](#rfc2548-2.1.6-1)

Otherwise, the LM-New-Password and LM-Old-Password fields MUST be used. (§2.1.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.1.6-1, so no unit is bound to it.

### [`RFC2548-2.1.7-1`](#rfc2548-2.1.7-1)

The value of this field MUST be identical to that in the Ident field in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT- Enc-PW and MS-CHAP-PW-2 attributes contained in a single Access- Request packet. (§2.1.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.1.7-1, so no unit is bound to it.

### [`RFC2548-2.1.7-2`](#rfc2548-2.1.7-2)

If least significant bit of the field is zero, the NT-Response field MUST be ignored and the LM- Response field used instead. (§2.1.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.1.7-2, so no unit is bound to it.

### [`RFC2548-2.1.7-3`](#rfc2548-2.1.7-3)

If this bit is set, at least one instance of the MS-CHAP-LM-Enc-PW attribute MUST be included in the packet. (§2.1.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.1.7-3, so no unit is bound to it.

### [`RFC2548-2.1.8-1`](#rfc2548-2.1.8-1)

The value of this field MUST be identical in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT-Enc-PW and MS- CHAP-PW-2 attributes which are present in the same Access-Request packet. (§2.1.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.1.8-1, so no unit is bound to it.

### [`RFC2548-2.2-1`](#rfc2548-2.2-1)

The value of this field MUST be identical in all instances of the MS-CHAP-LM-Enc-PW, MS-CHAP-NT-Enc-PW and MS- CHAP-PW-2 attributes which are present in the same Access-Request packet. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.2-1, so no unit is bound to it.

### [`RFC2548-2.3.2-1`](#rfc2548-2.3.2-1)

It is reserved for future use and MUST be zero. (§2.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.3.2-1, so no unit is bound to it.

### [`RFC2548-2.3.2-2`](#rfc2548-2.3.2-2)

Reserved This field is reserved for future use and MUST be zero. (§2.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.3.2-2, so no unit is bound to it.

### [`RFC2548-2.3.3-1`](#rfc2548-2.3.3-1)

This string MUST be included in the Message field of the MS-CHAP- V2 Success packet sent from the NAS to the peer. (§2.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.3.3-1, so no unit is bound to it.

### [`RFC2548-2.3.4-1`](#rfc2548-2.3.4-1)

The value of this field MUST be identical to that in the Ident field in all instances of the MS-CHAP-NT-Enc-PW contained in the Access-Request packet. (§2.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.3.4-1, so no unit is bound to it.

### [`RFC2548-2.3.4-2`](#rfc2548-2.3.4-2)

This field is reserved for future use and MUST be zero. (§2.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.3.4-2, so no unit is bound to it.

### [`RFC2548-2.4.1-1`](#rfc2548-2.4.1-1)

The Keys field MUST be encrypted by the RADIUS server using the same method defined for the User-Password Attribute [3]. (§2.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.1-1, so no unit is bound to it.

### [`RFC2548-2.4.2-1`](#rfc2548-2.4.2-1)

The most significant bit (leftmost) of the Salt field MUST be set (1). (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.2-1, so no unit is bound to it.

### [`RFC2548-2.4.2-2`](#rfc2548-2.4.2-2)

The contents of each Salt field in a given Access-Accept packet MUST be unique. (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.2-2, so no unit is bound to it.

### [`RFC2548-2.4.2-3`](#rfc2548-2.4.2-3)

If the combined length (in octets) of the unencrypted Key-Length and Key sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.2-3, so no unit is bound to it.

### [`RFC2548-2.4.2-4`](#rfc2548-2.4.2-4)

The String field MUST be encrypted as follows, prior to transmission: (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.2-4, so no unit is bound to it.

### [`RFC2548-2.4.2-5`](#rfc2548-2.4.2-5)

When the RADIUS server receives the attribute from the external server, it MUST correctly set the Salt field and encrypt the String field before transmitting it to the RADIUS client. (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.2-5, so no unit is bound to it.

### [`RFC2548-2.4.2-6`](#rfc2548-2.4.2-6)

If the channel used to communicate the MS-MPPE-Send-Key attribute is not secure from eavesdropping, the attribute MUST be cryptographically protected. (§2.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.2-6, so no unit is bound to it.

### [`RFC2548-2.4.3-1`](#rfc2548-2.4.3-1)

The most significant bit (leftmost) of the Salt field MUST be set (1). (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.3-1, so no unit is bound to it.

### [`RFC2548-2.4.3-2`](#rfc2548-2.4.3-2)

The contents of each Salt field in a given Access-Accept packet MUST be unique. (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.3-2, so no unit is bound to it.

### [`RFC2548-2.4.3-3`](#rfc2548-2.4.3-3)

If the combined length (in octets) of the unencrypted Key-Length and Key sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.3-3, so no unit is bound to it.

### [`RFC2548-2.4.3-4`](#rfc2548-2.4.3-4)

The String field MUST be encrypted as follows, prior to transmission: (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.3-4, so no unit is bound to it.

### [`RFC2548-2.4.3-5`](#rfc2548-2.4.3-5)

When the RADIUS server receives the attribute from the external server, it MUST correctly set the Salt field and encrypt the String field before transmitting it to the RADIUS client. (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.3-5, so no unit is bound to it.

### [`RFC2548-2.4.3-6`](#rfc2548-2.4.3-6)

If the channel used to communicate the MS-MPPE-Recv-Key attribute is not secure from eavesdropping, the attribute MUST be cryptographically protected. (§2.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.3-6, so no unit is bound to it.

### [`RFC2548-2.4.4-1`](#rfc2548-2.4.4-1)

If the Policy field is equal to 2 (Encryption-Required), any of the encryption types specified in the MS-MPPE-Encryption-Types Attribute MAY be used, but at least one MUST be used. (§2.4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.4.4-1, so no unit is bound to it.

### [`RFC2548-2.7.3-1`](#rfc2548-2.7.3-1)

If multiple MS-Filter Attributes are contained within a packet, they MUST be in order and they MUST be consecutive attributes in the packet. (§2.7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.7.3-1, so no unit is bound to it.

### [`RFC2548-2.7.3-2`](#rfc2548-2.7.3-2)

If multiple MS-Filter Attributes occur in a single Access-Accept packet, the Filter field from each MUST be concatenated in the order received to form the actual filter. (§2.7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-2.7.3-2, so no unit is bound to it.

### [`RFC2548-3-1`](#rfc2548-3-1)

0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2548-3-1, so no unit is bound to it.

### [`RFC2548-2.7.6-1`](#rfc2548-2.7.6-1)

The MS-Primary-DNS-Server Attribute is used to indicate the address of the primary Domain Name Server (DNS) [16, 17] server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtractMSDNSServers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2548_extract_dns_test.go#L15) | unit/verify | mutant, verified |
| positive | [`TestPoolIPCPDNSFromRadius`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/pool/rfc2548_dns_source_test.go#L51) | unit/verify | mutant, verified |

### [`RFC2548-2.7.7-1`](#rfc2548-2.7.7-1)

The MS-Secondary-DNS-Server Attribute is used to indicate the address of the secondary DNS server to be used by the PPP peer. It MAY be included in both Access-Accept and Accounting-Request packets. (§2.7.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtractMSDNSServers`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/authradius/rfc2548_extract_dns_test.go#L19) | unit/verify | mutant, verified |
| positive | [`TestPoolIPCPDNSFromRadius`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/plugins/pool/rfc2548_dns_source_test.go#L55) | unit/verify | mutant, verified |

## Extraction sign-off

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
| `front` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.3` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.5` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.6` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.7` | not stated | 3 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.1.8` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.2` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3.2` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3.3` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.3.4` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.1` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.2` | not stated | 6 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.3` | not stated | 6 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.4` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.4.5` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.5` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.5.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.5.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.5.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6.3` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.6.4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.1` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.2` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.3` | not stated | 2 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.5` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.6` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.7` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.8` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `2.7.9` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3` | not stated | 1 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `10` | not stated | 0 | walked | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

The walk over RFC 2548 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 2548, so its obligations are stated where they were written.
