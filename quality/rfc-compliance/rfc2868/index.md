# RFC 2868 - RADIUS Attributes for Tunnel Protocol Support

RADIUS tunnel attributes. Every requirement listed for RFC 2868, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 18 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 18 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 18 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 18 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 18 | of 18 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 18 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 18 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 18 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 18 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 18 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 18 | of 61 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (out-of-scope), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 18 mandatory requirements: they add up to 18. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | RADIUS tunnel attributes |
| Automatic checks | Excluded (out-of-scope) |
| Requirements | 61 |
| Mandatory requirements (MUST level), not checked | 18 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 18 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc2868.md` |
| Requirement table | `rfc/requirements/rfc2868.md` |
| RFC text | `rfc/full/rfc2868.txt` |

## Automatic checks

Excluded from automatic checks (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): OUT OF SCOPE as a document by owner decision, 2026-10-26: the owner rule committed in 1e7a9dbc34 (ai/META-DECISIONS.md, Goal and scope) counts an RFC a profile lists as optional only once Thomas brings it into scope, and this summary's `Optional use cases` row lists this RFC in the optional tier of the lns profile. The extraction is COMPLETE: the source text is at rfc/full/rfc2868.txt, every capitalised MUST-level sentence is a row of this summary or an exclusion in rfc/extraction/rfc2868.json (walked 2026-10-26), and none is gated. Ze's radius dictionary holds none of the tunnel attributes (types 64 to 67, 69, 81, 82, 83, 90, 91; internal/component/radius/dict.go), and the LNS reads none from an Access-Accept.

## Published support status

**Area:** RADIUS tunnel attributes

**Test verification:** 0 of 18 MUST proven

**Evidence:** -

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 18 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **18** | every mandatory requirement appears in exactly one group above |

**No test and no annotation (18):** [`RFC2868-3.1-1`](#rfc2868-3.1-1), [`RFC2868-3.1-2`](#rfc2868-3.1-2), [`RFC2868-3.2-1`](#rfc2868-3.2-1), [`RFC2868-3.3-1`](#rfc2868-3.3-1), [`RFC2868-3.3-2`](#rfc2868-3.3-2), [`RFC2868-3.4-1`](#rfc2868-3.4-1), [`RFC2868-3.4-2`](#rfc2868-3.4-2), [`RFC2868-3.4-3`](#rfc2868-3.4-3), [`RFC2868-3.5-1`](#rfc2868-3.5-1), [`RFC2868-3.5-2`](#rfc2868-3.5-2), [`RFC2868-3.5-3`](#rfc2868-3.5-3), [`RFC2868-3.5-4`](#rfc2868-3.5-4), [`RFC2868-3.7-1`](#rfc2868-3.7-1), [`RFC2868-3.8-1`](#rfc2868-3.8-1), [`RFC2868-3.9-1`](#rfc2868-3.9-1), [`RFC2868-3.10-1`](#rfc2868-3.10-1), [`RFC2868-4-1`](#rfc2868-4-1), [`RFC2868-5-1`](#rfc2868-5-1)

## Figures

- **MUST bar**: Not assessed 18
- **Overview bar**: Not assessed 61

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 18 |
| MUST rows kept (the MUST bar) | 18 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 18 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 43 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 43 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc2868.json](../../../data/rfc/rfc2868.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2868-3.1-1` | A tunnel initiator is not required to implement any of these tunnel types; if a tunnel initiator receives an Access-Accept packet which contains only unknown or unsupported Tunnel-Types, the tunnel initiator MUST behave as though an Access-Reject had been received instead. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.1-2` | If the Tag field is unused, it MUST be zero (0x00). (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.2-1` | If the Tag field is unused, it MUST be zero (0x00). (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.3-1` | Conformant implementations MUST support the dotted-decimal format and SHOULD support the FQDN format for IP addresses. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.3-2` | Conformant implementations MUST support the preferred form and SHOULD support both the alternate text form and the FQDN format for IPv6 addresses. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.4-1` | The Tunnel-Server-Endpoint Attribute MAY be included (as a hint to the RADIUS server) in the Access-Request packet and MUST be included in the Access-Accept packet if the initiation of a tunnel is desired. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.4-2` | Conformant implementations MUST support the dotted-decimal format and SHOULD support the FQDN format for IP addresses. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.4-3` | Conformant implementations MUST support the preferred form and SHOULD support both the alternate text form and the FQDN format for IPv6 addresses. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.5-1` | The most significant bit (leftmost) of the Salt field MUST be set (1). (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.5-2` | The contents of each Salt field in a given Access-Accept packet MUST be unique. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.5-3` | If the combined length (in octets) of the unencrypted Data-Length and Password sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.5-4` | The String field MUST be encrypted as follows, prior to transmission: (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.7-1` | A tunnel initiator MUST NOT assign a session for which a Tunnel-Assignment-ID Attribute was not specified to a named tunnel (i.e. one that was initiated by a session specifying this attribute). (§3.7) | MUST NOT | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.8-1` | If the Tag field is unused, it MUST be zero (0x00). (§3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.9-1` | The Tunnel-Client-Auth-ID Attribute MAY be included (as a hint to the RADIUS server) in the Access-Request packet, and MUST be included in the Access-Accept packet if an authentication name other than the default is desired. (§3.9) | MUST | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.10-1` | The Tunnel-Client-Auth-ID Attribute MAY be included (as a hint to the RADIUS server) in the Access-Request packet, and MUST be included in the Access-Accept packet if an authentication name other than the default is desired. (§3.10) | MUST | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-4-1` | 0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-5-1` | In particular, end-users SHOULD NOT rely upon the security of the tunnel to protect their data; encryption and/or integrity protection of tunneled traffic MUST NOT be considered as a replacement for end-to-end security. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3-1` | In this case, the attributes to be applied to any given tunnel SHOULD all contain the same value in their respective Tag fields; otherwise, the Tag field SHOULD NOT be used. (§3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3-2` | If the RADIUS server returns attributes describing multiple tunnels then the tunnels SHOULD be interpreted by the tunnel initiator as alternatives and the server SHOULD include an instance of the Tunnel-Preference Attribute in the set of Attributes pertaining to each alternative tunnel. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3-3` | Similarly, if the RADIUS client includes multiple sets of tunnel Attributes in an Access-Request packet, all the Attributes pertaining to a given tunnel SHOULD contain the same value in their respective Tag fields and each set SHOULD include an appropriately valued instance of the Tunnel-Preference Attribute. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.1-4` | If the Tunnel-Type Attribute is present in an Access-Request packet sent from a tunnel initiator, it SHOULD be taken as a hint to the RADIUS server as to the tunnelling protocols supported by the tunnel end-point; the RADIUS server MAY ignore the hint, however. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.1-5` | If the Tunnel-Type Attribute is present in an Access-Request packet sent from a tunnel terminator, it SHOULD be taken to signify the tunnelling protocol in use. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.1-7` | If a tunnel terminator receives an Access-Accept packet which contains one or more Tunnel-Type Attributes, none of which represent the tunneling protocol in use, the tunnel terminator SHOULD behave as though an Access-Reject had been received instead. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.2-2` | It MAY be included in both Access-Request and Access-Accept packets; if it is present in an Access-Request packet, it SHOULD be taken as a hint to the RADIUS server as to the tunnel media supported by the tunnel end- point. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.3-4` | This Attribute SHOULD be included in Accounting- Request packets which contain Acct-Status-Type attributes with values of either Start or Stop, in which case it indicates the address from which the tunnel was initiated. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.3-5` | If the value of the Tag field is greater than 0x00 and less than or equal to 0x1F, it SHOULD be interpreted as indicating which tunnel (of several alternatives) this attribute pertains. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.3-6` | If the Tag field is greater than 0x1F, it SHOULD be interpreted as the first byte of the following String field. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.4-4` | It SHOULD be included in Accounting-Request packets which contain Acct-Status-Type attributes with values of either Start or Stop and which pertain to a tunneled session. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.4-5` | If the value of the Tag field is greater than 0x00 and less than or equal to 0x1F, it SHOULD be interpreted as indicating which tunnel (of several alternatives) this attribute pertains. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.4-6` | If the Tag field is greater than 0x1F, it SHOULD be interpreted as the first byte of the following String field. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.5-5` | If the value of the Tag field is greater than 0x00 and less than or equal to 0x1F, it SHOULD be interpreted as indicating which tunnel (of several alternatives) this attribute pertains; otherwise, the Tag field SHOULD be ignored. (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.6-1` | The Tunnel-Private-Group-ID Attribute MAY be included in the Access-Request packet if the tunnel initiator can pre- determine the group resulting from a particular connection and SHOULD be included in the Access-Accept packet if this tunnel session is to be treated as belonging to a particular private group. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.6-2` | It SHOULD be included in Accounting-Request packets which contain Acct-Status-Type attributes with values of either Start or Stop and which pertain to a tunneled session. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.6-3` | If the value of the Tag field is greater than 0x00 and less than or equal to 0x1F, it SHOULD be interpreted as indicating which tunnel (of several alternatives) this attribute pertains. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.6-4` | If the Tag field is greater than 0x1F, it SHOULD be interpreted as the first byte of the following String field. (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.7-4` | This attribute SHOULD also be included in Accounting-Request packets which contain Acct- Status-Type attributes with values of either Start or Stop and which pertain to a tunneled session. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.7-5` | If the value of the Tag field is greater than 0x00 and less than or equal to 0x1F, it SHOULD be interpreted as indicating which tunnel (of several alternatives) this attribute pertains. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.7-6` | If the Tag field is greater than 0x1F, it SHOULD be interpreted as the first byte of the following String field. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.8-2` | Description If more than one set of tunneling attributes is returned by the RADIUS server to the tunnel initiator, this Attribute SHOULD be included in each set to indicate the relative preference assigned to each tunnel. (§3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.8-3` | If, however, it supports both tunnel protocols, it SHOULD use the value of the Tunnel-Preference Attribute to decide which tunnel should be started. (§3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.8-4` | The tunnel having the numerically lowest value in the Value field of this Attribute SHOULD be given the highest preference. (§3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.8-6` | In this case, the tunnel initiator SHOULD use locally configured metrics to decide which set of attributes to use. (§3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.9-2` | This Attribute SHOULD be included in Accounting-Request packets which contain Acct-Status-Type attributes with values of either Start or Stop and which pertain to a tunneled session. (§3.9) | SHOULD | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.9-3` | If the value of the Tag field is greater than 0x00 and less than or equal to 0x1F, it SHOULD be interpreted as indicating which tunnel (of several alternatives) this attribute pertains. (§3.9) | SHOULD | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.9-4` | If the Tag field is greater than 0x1F, it SHOULD be interpreted as the first byte of the following String field. (§3.9) | SHOULD | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.9-5` | The authentication name SHOULD be represented in the UTF-8 charset. (§3.9) | SHOULD | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.10-2` | This Attribute SHOULD be included in Accounting-Request packets which contain Acct-Status-Type attributes with values of either Start or Stop and which pertain to a tunneled session. (§3.10) | SHOULD | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.10-3` | If the value of the Tag field is greater than 0x00 and less than or equal to 0x1F, it SHOULD be interpreted as indicating which tunnel (of several alternatives) this attribute pertains. (§3.10) | SHOULD | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.10-4` | If the Tag field is greater than 0x1F, it SHOULD be interpreted as the first byte of the following String field. (§3.10) | SHOULD | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.10-5` | The authentication name SHOULD be represented in the UTF-8 charset. (§3.10) | SHOULD | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-5-2` | For this reason, the Tunnel- Password Attribute SHOULD NOT be included in Access-Accept packets which may pass through (relatively) untrusted RADIUS proxies. (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-5-3` | In addition, the Tunnel-Password Attribute SHOULD NOT be returned to an unauthenticated client; if the corresponding Access-Request packet did not contain a verified instance of the Signature Attribute [15], the Access-Accept packet SHOULD NOT contain an instance of the Tunnel-Password Attribute. (§5) | SHOULD NOT | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.1-3` | It MAY be included in Access-Request, Access-Accept and Accounting-Request packets. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.1-6` | In this case, if the RADIUS server determines that the use of the communicated protocol is not authorized, it MAY return an Access-Reject packet. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.2-3` | The RADIUS server MAY ignore the hint, however. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.3-3` | It MAY be included in both Access-Request and Access- Accept packets to indicate the address from which a new tunnel is to be initiated. (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.7-2` | This attribute MAY be included in the Access-Accept. (§3.7) | MAY | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.7-3` | The tunnel initiator receiving this attribute MAY choose to ignore it and assign the session to an arbitrary multiplexed or non-multiplexed tunnel between the desired endpoints. (§3.7) | MAY | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.8-5` | The values assigned to two or more instances of the Tunnel-Preference Attribute within a given Access-Accept packet MAY be identical. (§3.8) | MAY | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2868-3.8-7` | This Attribute MAY be included (as a hint to the server) in Access-Request packets, but the RADIUS server is not required to honor this hint. (§3.8) | MAY | 3.8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2868-3.1-1`](#rfc2868-3.1-1) A tunnel initiator is not required to implement any of these tunnel types; if a tunnel initiator receives an Access-Accept packet which contains only unknown or unsupported Tunnel-Types, the tunnel initiator MUST behave as though an Access-Reject had been received instead. (§3.1) | no test | no test carries this requirement id |
| [`RFC2868-3.1-2`](#rfc2868-3.1-2) If the Tag field is unused, it MUST be zero (0x00). (§3.1) | no test | no test carries this requirement id |
| [`RFC2868-3.2-1`](#rfc2868-3.2-1) If the Tag field is unused, it MUST be zero (0x00). (§3.2) | no test | no test carries this requirement id |
| [`RFC2868-3.3-1`](#rfc2868-3.3-1) Conformant implementations MUST support the dotted-decimal format and SHOULD support the FQDN format for IP addresses. (§3.3) | no test | no test carries this requirement id |
| [`RFC2868-3.3-2`](#rfc2868-3.3-2) Conformant implementations MUST support the preferred form and SHOULD support both the alternate text form and the FQDN format for IPv6 addresses. (§3.3) | no test | no test carries this requirement id |
| [`RFC2868-3.4-1`](#rfc2868-3.4-1) The Tunnel-Server-Endpoint Attribute MAY be included (as a hint to the RADIUS server) in the Access-Request packet and MUST be included in the Access-Accept packet if the initiation of a tunnel is desired. (§3.4) | no test | no test carries this requirement id |
| [`RFC2868-3.4-2`](#rfc2868-3.4-2) Conformant implementations MUST support the dotted-decimal format and SHOULD support the FQDN format for IP addresses. (§3.4) | no test | no test carries this requirement id |
| [`RFC2868-3.4-3`](#rfc2868-3.4-3) Conformant implementations MUST support the preferred form and SHOULD support both the alternate text form and the FQDN format for IPv6 addresses. (§3.4) | no test | no test carries this requirement id |
| [`RFC2868-3.5-1`](#rfc2868-3.5-1) The most significant bit (leftmost) of the Salt field MUST be set (1). (§3.5) | no test | no test carries this requirement id |
| [`RFC2868-3.5-2`](#rfc2868-3.5-2) The contents of each Salt field in a given Access-Accept packet MUST be unique. (§3.5) | no test | no test carries this requirement id |
| [`RFC2868-3.5-3`](#rfc2868-3.5-3) If the combined length (in octets) of the unencrypted Data-Length and Password sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§3.5) | no test | no test carries this requirement id |
| [`RFC2868-3.5-4`](#rfc2868-3.5-4) The String field MUST be encrypted as follows, prior to transmission: (§3.5) | no test | no test carries this requirement id |
| [`RFC2868-3.7-1`](#rfc2868-3.7-1) A tunnel initiator MUST NOT assign a session for which a Tunnel-Assignment-ID Attribute was not specified to a named tunnel (i.e. one that was initiated by a session specifying this attribute). (§3.7) | no test | no test carries this requirement id |
| [`RFC2868-3.8-1`](#rfc2868-3.8-1) If the Tag field is unused, it MUST be zero (0x00). (§3.8) | no test | no test carries this requirement id |
| [`RFC2868-3.9-1`](#rfc2868-3.9-1) The Tunnel-Client-Auth-ID Attribute MAY be included (as a hint to the RADIUS server) in the Access-Request packet, and MUST be included in the Access-Accept packet if an authentication name other than the default is desired. (§3.9) | no test | no test carries this requirement id |
| [`RFC2868-3.10-1`](#rfc2868-3.10-1) The Tunnel-Client-Auth-ID Attribute MAY be included (as a hint to the RADIUS server) in the Access-Request packet, and MUST be included in the Access-Accept packet if an authentication name other than the default is desired. (§3.10) | no test | no test carries this requirement id |
| [`RFC2868-4-1`](#rfc2868-4-1) 0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§4) | no test | no test carries this requirement id |
| [`RFC2868-5-1`](#rfc2868-5-1) In particular, end-users SHOULD NOT rely upon the security of the tunnel to protect their data; encryption and/or integrity protection of tunneled traffic MUST NOT be considered as a replacement for end-to-end security. (§5) | no test | no test carries this requirement id |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC2868-3.1-1`](#rfc2868-3.1-1)

A tunnel initiator is not required to implement any of these tunnel types; if a tunnel initiator receives an Access-Accept packet which contains only unknown or unsupported Tunnel-Types, the tunnel initiator MUST behave as though an Access-Reject had been received instead. (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.1-1, so there is no linked test.

### [`RFC2868-3.1-2`](#rfc2868-3.1-2)

If the Tag field is unused, it MUST be zero (0x00). (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.1-2, so there is no linked test.

### [`RFC2868-3.2-1`](#rfc2868-3.2-1)

If the Tag field is unused, it MUST be zero (0x00). (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.2-1, so there is no linked test.

### [`RFC2868-3.3-1`](#rfc2868-3.3-1)

Conformant implementations MUST support the dotted-decimal format and SHOULD support the FQDN format for IP addresses. (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.3-1, so there is no linked test.

### [`RFC2868-3.3-2`](#rfc2868-3.3-2)

Conformant implementations MUST support the preferred form and SHOULD support both the alternate text form and the FQDN format for IPv6 addresses. (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.3-2, so there is no linked test.

### [`RFC2868-3.4-1`](#rfc2868-3.4-1)

The Tunnel-Server-Endpoint Attribute MAY be included (as a hint to the RADIUS server) in the Access-Request packet and MUST be included in the Access-Accept packet if the initiation of a tunnel is desired. (§3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.4-1, so there is no linked test.

### [`RFC2868-3.4-2`](#rfc2868-3.4-2)

Conformant implementations MUST support the dotted-decimal format and SHOULD support the FQDN format for IP addresses. (§3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.4-2, so there is no linked test.

### [`RFC2868-3.4-3`](#rfc2868-3.4-3)

Conformant implementations MUST support the preferred form and SHOULD support both the alternate text form and the FQDN format for IPv6 addresses. (§3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.4-3, so there is no linked test.

### [`RFC2868-3.5-1`](#rfc2868-3.5-1)

The most significant bit (leftmost) of the Salt field MUST be set (1). (§3.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.5-1, so there is no linked test.

### [`RFC2868-3.5-2`](#rfc2868-3.5-2)

The contents of each Salt field in a given Access-Accept packet MUST be unique. (§3.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.5-2, so there is no linked test.

### [`RFC2868-3.5-3`](#rfc2868-3.5-3)

If the combined length (in octets) of the unencrypted Data-Length and Password sub-fields is not an even multiple of 16, then the Padding sub-field MUST be present. (§3.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.5-3, so there is no linked test.

### [`RFC2868-3.5-4`](#rfc2868-3.5-4)

The String field MUST be encrypted as follows, prior to transmission: (§3.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.5-4, so there is no linked test.

### [`RFC2868-3.7-1`](#rfc2868-3.7-1)

A tunnel initiator MUST NOT assign a session for which a Tunnel-Assignment-ID Attribute was not specified to a named tunnel (i.e. one that was initiated by a session specifying this attribute). (§3.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.7-1, so there is no linked test.

### [`RFC2868-3.8-1`](#rfc2868-3.8-1)

If the Tag field is unused, it MUST be zero (0x00). (§3.8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.8-1, so there is no linked test.

### [`RFC2868-3.9-1`](#rfc2868-3.9-1)

The Tunnel-Client-Auth-ID Attribute MAY be included (as a hint to the RADIUS server) in the Access-Request packet, and MUST be included in the Access-Accept packet if an authentication name other than the default is desired. (§3.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.9-1, so there is no linked test.

### [`RFC2868-3.10-1`](#rfc2868-3.10-1)

The Tunnel-Client-Auth-ID Attribute MAY be included (as a hint to the RADIUS server) in the Access-Request packet, and MUST be included in the Access-Accept packet if an authentication name other than the default is desired. (§3.10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-3.10-1, so there is no linked test.

### [`RFC2868-4-1`](#rfc2868-4-1)

0 This attribute MUST NOT be present in packet. 0+ Zero or more instances of this attribute MAY be present in packet. 0-1 Zero or one instance of this attribute MAY be present in packet. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-4-1, so there is no linked test.

### [`RFC2868-5-1`](#rfc2868-5-1)

In particular, end-users SHOULD NOT rely upon the security of the tunnel to protect their data; encryption and/or integrity protection of tunneled traffic MUST NOT be considered as a replacement for end-to-end security. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2868-5-1, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc2868.txt |
| Source fingerprint | b63be609d86aa086 |
| Record | rfc/extraction/rfc2868.json |
| Mapped sentences | 18 |
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
| `3.1` | not stated | 2 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.2` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.3` | not stated | 2 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.4` | not stated | 3 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.5` | not stated | 4 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.6` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.7` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.8` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.9` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `3.10` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `4` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `5` | not stated | 1 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.1` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `6.2` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `7` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `8` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `9` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `10` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |
| `11` | not stated | 0 | reviewed | Walked against the source text; every capitalised keyword sentence is captured in the checklist or decided at its site. |

### Excluded sentences

The review of RFC 2868 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 2868, so its obligations are stated where they were written.
