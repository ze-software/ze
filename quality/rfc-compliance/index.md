# RFC Compliance Gate Report

Source: `internal/le/rfc`, `rfc/short/*.md`, and `rfc/audit/*.json`.

## Gate verdict

RED. 153 open gate issues. Check results below names them, up to the 25 this page inlines. Reproduce it with `./le rfc check`. The gate's own line reads `rfc-requirements: 153 violation(s)`.

### Overall

the populations every share below is taken over

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3,264 | 5,384 extracted from 198 summaries | MUST-level requirements the gate HOLDS, across 181 enrolled RFCs. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 834 | of 3,264 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 58.1% | 1,411 of 2,430 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 15.2% | 370 of 2,430 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 2,430 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 3.4% | 139 of 4,059 tagged units in enrolled RFCs | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Semantic verdicts | 50 | 0 shifted, 2 stale, 3,212 missing | requirements a reader has judged and whose judgement is still current. A missing verdict is not claimed, and the shifted and stale ones are named on their own RFC's page |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 26.7% | 649 of 2,430 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Gate verdict | RED | 153 open gate issues | whether ./le rfc check passes over this tree |

The 4 shares marked as a part above are the whole of the 2,430 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Gate verdict | bad | the verdict IS the value: green when the gate passes, red when it does not |
| Semantic verdicts | neutral | no color: a count of judgements recorded is a scale rather than an outcome, and the shifted and stale counts beside it are the states that need reading |

| Metric | Value |
|---|---:|
| Gate issues | 153 |
| Gated MUST-level requirements | 3,264 |
| Enrolled RFCs | 181 |
| Resolved test tags | 4,156 |
| Declared gaps | 501 |
| RFCs with declared gaps | 80 |
| Fresh semantic audit verdicts | 50 |
| Shifted semantic audit verdicts | 0 |
| Stale semantic audit verdicts | 2 |

## Requirement buckets

The bar is the 2,430 obligations that bind Ze. 834 further gated MUSTs are {not-applicable}: they do not bind Ze, they are not in the bar, and they are counted apart below.

| Bucket | Count | Share of binding | Source condition |
|---|---:|---:|---|
| Positive and negative tests | 1,411 | 58.1% | `positive tag + negative tag` |
| One polarity plus reason | 370 | 15.2% | `{single-polarity} annotation + required tag` |
| Declared gap | 501 | 20.6% | `{gap} annotation + public ledger disclosure` |
| One polarity, unexcused | 0 | 0.0% | `tag without annotation` |
| Missing, unexcused | 148 | 6.1% | `no tag, no annotation` |
| **Obligations that bind Ze** | **2,430** | 100.0% | every obligation that binds Ze falls in exactly one bucket above |
| Not applicable | 834 | - | `{not-applicable} annotation`: the obligation does not bind Ze, so it is scope rather than coverage |
| **Gated MUST-level requirements** | **3,264** | - | the accounting total: 2,430 that bind Ze plus 834 that do not |

## Gap disclosure

| Public status for RFCs with gaps | RFCs |
|---|---:|
| Partial | 60 |
| Experimental | 14 |
| Supported | 3 |
| Not supported | 1 |
| Unsupported | 1 |
| Supported within BMP sender scope | 1 |

### Supported rows that still disclose a gap

- **RFC 1350:** RFC1350-2-3 unmet (Sorcerer's Apprentice fix): sendAndWaitACK retransmits DATA on any non-matching ACK (handler.go) instead of silently ignoring a duplicate or stale ACK.
- **RFC 6396:** One MUST gap gated in rfc/short/rfc6396.md [RFC6396-4.4.3-1]: the live BGP4MP writer always emits the BGP4MP_MESSAGE_AS4 subtype and records the on-wire message verbatim without checking the session's negotiated 4-byte-AS capability, so a message from an OLD (2-byte) peer carries a 2-byte AS_PATH mislabeled as AS4. RIB-path AS_PATH is unaffected (canonicalized to 4-byte).
- **RFC 7313:** Four MUST-level receive-side gaps annotated in `rfc/short/rfc7313.md`: RFC7313-4-4/4-5 -- a received BoRR/EoRR is log-only (internal/component/bgp/plugins/rib/rib.go), so ze marks no Adj-RIB-In routes stale and purges none; and RFC7313-4-6/4-7 -- neither the send nor receive path applies a Graceful-Restart End-of-RIB gate to BoRR emission or acceptance.
- **RFC 8671:** One MUST is a gap. Section 6.2 requires the O flag to be zero on a Statistics Report, and ze sends no Statistics Report at all: `senderSession.writeStatisticsReport` clears the flag correctly and has no non-test caller, and the `statistics-timeout` leaf drives no timer, so the obligation is never exercised (RFC8671-6.2-1). One feature is absent by decision, and it is not a conformance gap: ze exports the post-policy Adj-RIB-Out view only, and the pre-policy Adj-RIB-Out view is not built. RFC 7854 Section 5 leaves that choice to the implementation, so the obligations that hang on it do not bind ze (RFC8671-5.2-1). A later scope decision can revisit it and build the pre-policy view.

## Exclusion disclosure

A reviewer walks an RFC's own text sentence by sentence and decides which sentences become requirements. One that does not is EXCLUDED, with a kind and a reason, and it never reaches the gated ledger at all. That is a different mechanism from the Out of scope card above, which counts requirements that exist and carry a {not-applicable} annotation. Across the sign-offs done so far, 857 sentences were mapped to a requirement and 508 were declined. 13 of those declines are not scope at all: they are obligations Ze OWES, relocated to a named spec, and they are stated apart below.

58 of 198 summaries carry an extraction sign-off, 56 of them among the 181 enrolled. The other 140 have no exclusion ledger at all, so what follows counts the walks that HAVE been done and is not the whole picture.

| Excluded kind | Means | Sites | Summaries | What it means |
|---|---|---|---|---|
| `binds-another-role` | never bound Ze | 223 | 21 | the obligation is addressed to a role Ze never acts as |
| `duplicate-of` | never bound Ze | 109 | 22 | the same obligation is already captured under another requirement id |
| `not-a-requirement` | never bound Ze | 96 | 35 | the sentence states a fact or describes another document, and directs no implementation |
| `feature-out-of-scope` | never bound Ze | 30 | 5 | the RFC makes a feature OPTIONAL, Ze decided not to offer it, and this obligation is conditional on offering it |
| `cross-document` | never bound Ze | 24 | 16 | the obligation belongs to another document that this one only cites |
| `advisory-in-context` | never bound Ze | 13 | 7 | the sentence advises on applying a rule stated elsewhere and adds no obligation of its own |
| `relocated-to-spec` | Ze owes it | 13 | 2 | the obligation is real and unbuilt, and a named spec owes it |
| **Sentences declined** | - | **508** | - | 495 say the obligation never bound Ze and 13 say Ze owes it, of 1,365 normative sentences the walks found |

**binds-another-role (223):** [`RFC 1035`](rfc1035/index.md), [`RFC 1350`](rfc1350/index.md), [`RFC 1997`](rfc1997/index.md), [`RFC 2545`](rfc2545/index.md), [`RFC 2759`](rfc2759/index.md), [`RFC 2865`](rfc2865/index.md), [`RFC 2866`](rfc2866/index.md), [`RFC 2869`](rfc2869/index.md), [`RFC 3032`](rfc3032/index.md), [`RFC 3748`](rfc3748/index.md), [`RFC 4302`](rfc4302/index.md), [`RFC 4303`](rfc4303/index.md), [`RFC 4360`](rfc4360/index.md), [`RFC 4364`](rfc4364/index.md), [`RFC 4456`](rfc4456/index.md), [`RFC 4761`](rfc4761/index.md), [`RFC 5176`](rfc5176/index.md), [`RFC 6396`](rfc6396/index.md), [`RFC 7296`](rfc7296/index.md), [`RFC 7535`](rfc7535/index.md), [`RFC 8654`](rfc8654/index.md)

ai/rules/rfc-compliance.md treats binds-another-role as PRESUMED WRONG until it is justified: Ze rarely implements one side of a protocol, so an obligation addressed to "the sender" or "the receiver" almost always binds it, and the label reads as "not our problem" where the truth is usually "our problem, unbuilt". Each one is justified on its own RFC's page, under Extraction sign-off, and the justification MUST name the role, show Ze never acts as it, and cite the producer that would act as it if Ze did.

**duplicate-of (109):** [`RFC 1035`](rfc1035/index.md), [`RFC 2759`](rfc2759/index.md), [`RFC 2865`](rfc2865/index.md), [`RFC 2866`](rfc2866/index.md), [`RFC 2869`](rfc2869/index.md), [`RFC 3032`](rfc3032/index.md), [`RFC 3748`](rfc3748/index.md), [`RFC 3948`](rfc3948/index.md), [`RFC 4302`](rfc4302/index.md), [`RFC 4303`](rfc4303/index.md), [`RFC 4364`](rfc4364/index.md), [`RFC 4456`](rfc4456/index.md), [`RFC 4760`](rfc4760/index.md), [`RFC 5549`](rfc5549/index.md), [`RFC 5798`](rfc5798/index.md), [`RFC 5880`](rfc5880/index.md), [`RFC 7296`](rfc7296/index.md), [`RFC 8671`](rfc8671/index.md), [`RFC 8950`](rfc8950/index.md), [`RFC 9003`](rfc9003/index.md), [`RFC 9069`](rfc9069/index.md), [`RFC 9234`](rfc9234/index.md)

**not-a-requirement (96):** [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY`](draft-abraitis-bgp-version-capability/index.md), [`DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY`](draft-walton-bgp-hostname-capability/index.md), [`RFC 1035`](rfc1035/index.md), [`RFC 1350`](rfc1350/index.md), [`RFC 2347`](rfc2347/index.md), [`RFC 2385`](rfc2385/index.md), [`RFC 2545`](rfc2545/index.md), [`RFC 2759`](rfc2759/index.md), [`RFC 2865`](rfc2865/index.md), [`RFC 2869`](rfc2869/index.md), [`RFC 2918`](rfc2918/index.md), [`RFC 3032`](rfc3032/index.md), [`RFC 3748`](rfc3748/index.md), [`RFC 3765`](rfc3765/index.md), [`RFC 3948`](rfc3948/index.md), [`RFC 4303`](rfc4303/index.md), [`RFC 4360`](rfc4360/index.md), [`RFC 4364`](rfc4364/index.md), [`RFC 4456`](rfc4456/index.md), [`RFC 4761`](rfc4761/index.md), [`RFC 5176`](rfc5176/index.md), [`RFC 5282`](rfc5282/index.md), [`RFC 5301`](rfc5301/index.md), [`RFC 5492`](rfc5492/index.md), [`RFC 5798`](rfc5798/index.md), [`RFC 6286`](rfc6286/index.md), [`RFC 6396`](rfc6396/index.md), [`RFC 7296`](rfc7296/index.md), [`RFC 7534`](rfc7534/index.md), [`RFC 7535`](rfc7535/index.md), [`RFC 7911`](rfc7911/index.md), [`RFC 7999`](rfc7999/index.md), [`RFC 8654`](rfc8654/index.md), [`RFC 9384`](rfc9384/index.md), [`RFC 9687`](rfc9687/index.md)

**feature-out-of-scope (30):** [`RFC 3748`](rfc3748/index.md), [`RFC 5082`](rfc5082/index.md), [`RFC 5176`](rfc5176/index.md), [`RFC 8671`](rfc8671/index.md), [`RFC 9069`](rfc9069/index.md)

**cross-document (24):** [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY`](draft-abraitis-bgp-version-capability/index.md), [`RFC 2545`](rfc2545/index.md), [`RFC 2869`](rfc2869/index.md), [`RFC 3032`](rfc3032/index.md), [`RFC 3748`](rfc3748/index.md), [`RFC 3948`](rfc3948/index.md), [`RFC 4303`](rfc4303/index.md), [`RFC 4364`](rfc4364/index.md), [`RFC 4761`](rfc4761/index.md), [`RFC 5082`](rfc5082/index.md), [`RFC 5176`](rfc5176/index.md), [`RFC 5282`](rfc5282/index.md), [`RFC 5492`](rfc5492/index.md), [`RFC 7535`](rfc7535/index.md), [`RFC 8654`](rfc8654/index.md), [`RFC 9069`](rfc9069/index.md)

**advisory-in-context (13):** [`RFC 1035`](rfc1035/index.md), [`RFC 2866`](rfc2866/index.md), [`RFC 3032`](rfc3032/index.md), [`RFC 3748`](rfc3748/index.md), [`RFC 3948`](rfc3948/index.md), [`RFC 4761`](rfc4761/index.md), [`RFC 5176`](rfc5176/index.md)

### Obligations relocated to a spec

These 13 sentences are NOT scope. Each is an obligation Ze owes and has not built, moved to a named spec that reserves a requirement id for it. ./le rfc check refuses the sign-off unless that spec exists and still reserves the id, so each row is tracked work rather than an obligation that went away.

| RFC | Reserved id | The obligation | The spec that owes it |
|---|---|---|---|
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-1.7-3` | Implementations that conform to this document MUST ignore proposals that have configuration attribute type 5, the old value for INTERNAL_ADDRESS_EXPIRY. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-2.19-2` | Initiator Responder ------------------------------------------------------------------- HDR, SK {IDi, [CERT,] [CERTREQ,] [IDr,] AUTH, CP(CFG_REQUEST), SAi2, TSi, TSr} --> <-- HDR, SK {IDr, [CERT,] AUTH, CP(CFG_REPLY), SAr2, TSi, TSr} In all cases, the CP payload MUST be inserted before the SA payload. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-2.19-3` | In variations of the protocol where there are multiple IKE_AUTH exchanges, the CP payloads MUST be inserted in the messages containing the SA payloads. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-2.19-5` | The responder MUST NOT send a CFG_REPLY without having first received a CP(CFG_REQUEST) from the initiator, because we do not want the IRAS to perform an unnecessary configuration lookup if the IRAC cannot process the REPLY. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-2.19-6` | In the case where the IRAS's configuration requires that CP be used for a given identity IDi, but IRAC has failed to send a CP(CFG_REQUEST), IRAS MUST fail the request, and terminate the Child SA creation with a FAILED_CP_REQUIRED error. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-2.22-1` | These payloads MUST NOT occur in messages that do not contain SA payloads. | `plan/spec-ipsec-ipcomp.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-2.22-2` | Although there has been discussion of allowing multiple compression algorithms to be accepted and to have different compression algorithms available for the two directions of a Child SA, implementations of this specification MUST NOT accept an IPComp algorithm that was not proposed, MUST NOT accept more than one, and MUST NOT compress using an algorithm other than one proposed and accepted in the setup of the Child SA. | `plan/spec-ipsec-ipcomp.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-3.15.1-1` | Only one netmask is allowed in the request and response messages (e.g., 255.255.255.0), and it MUST be used only with an INTERNAL_IP4_ADDRESS attribute. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-3.15.1-3` | o SUPPORTED_ATTRIBUTES - When used within a Request, this attribute MUST be zero-length and specifies a query to the responder to reply back with all of the attributes that it supports. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-3.15.1-4` | Unrecognized or unsupported attributes MUST be ignored in both requests and responses. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-4-2` | If an implementation supports responding to such requests, it MUST parse the CP payload of type CFG_REQUEST in the first message in the IKE_AUTH exchange and recognize a field of type INTERNAL_IP4_ADDRESS or INTERNAL_IP6_ADDRESS. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7296`](rfc7296/index.md) | `RFC7296-4-3` | If it supports leasing an address of the appropriate type, it MUST return a CP payload of type CFG_REPLY containing an address of the requested type. | `plan/spec-ipsec-remote-access.md` |
| [`RFC 7947`](rfc7947/index.md) | `RFC7947-2.1-1` | A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In. | `plan/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md` |

## Top gap clusters

| RFC | Declared gaps | Public status |
|---|---:|---|
| `RFC 9012` | 51 | Partial |
| `DRAFT-IETF-BESS-MUP-SAFI` | 37 | Partial |
| `RFC 1661` | 24 | Partial |
| `RFC 9830` | 20 | Partial |
| `RFC 2131` | 18 | Partial |
| `RFC 7166` | 17 | Unsupported |
| `RFC 4577` | 16 | Not supported |
| `RFC 4271` | 15 | Partial |
| `RFC 7432` | 15 | Partial |
| `RFC 5880` | 14 | Partial |
| `RFC 8665` | 14 | Partial |
| `RFC 9514` | 13 | Partial |

## How this is checked

This gate runs before a commit is verified: ./le rfc check is 1 stage of the 43 that ./le verify current mode full runs.

| Input | Producer | What it answered here |
|---|---|---|
| Reproduce it | `./le rfc check` | 1 of 43 full-mode verify stages run it |
| Requirement source | `rfc/short/*.md` | 3,264 gated MUST-level requirements |
| Enrolment | `rfc/short/*.md`, the `\| Enrolment \|` Meta row | 181 enrolled RFCs |
| Test tags | `internal/`, `pkg/`, `test/` | 4,156 resolved tags |
| Public ledger | `rfc/short/*.md`, the `\| Support \|` Meta row | 80 RFCs with gaps, 4 Supported with Remaining |
| Semantic audits | `rfc/audit/*.json` | 50 fresh, 0 shifted, 2 stale, 3,212 missing |
| Pre-commit verification | `internal/le/verify/engine/stages.go` | ./le rfc check, 1 of 43 full-mode stages |
| Published artifacts | `data/rfc-compliance.json`, `data/rfc-requirements.json` | the same answers this page renders, machine-readable |

## Check results

| RFC | Requirement | Level | What is wrong | The requirement |
|---|---|---|---|---|
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-3-1) | MUST | has no test and no annotation | "If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the configuration option half (§3) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-3-2) | MUST | has no test and no annotation | "If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use, and MUST default to disabled" -- the default-to-disabled half (§3) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-3-3) | MUST | has no test and no annotation | "The Capability Length for the Software Version Capability MUST be greater than zero" (§3) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-3-4) | SHALL | has no test and no annotation | "A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the encoding-error half (§3) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-3-5) | MUST | has no test and no annotation | "A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored" -- the ignore half (§3) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-3-6) | MUST | has no test and no annotation | "The Version field MUST be encoded using UTF-8" (§3) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-3-7) | MUST NOT | has no test and no annotation | "A receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences" (§3) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-3-8) | MUST NOT | has no test and no annotation | "A sender SHOULD limit generated product identifiers to what is necessary to identify the product; a sender MUST NOT generate advertising or other nonessential information within the product identifier" -- the MUST NOT half (§3) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-3.1-1) | REQUIRED | has no test and no annotation | "Implementations of this specification are REQUIRED Extended Optional Parameters Length for BGP OPEN Message support as defined in [RFC9072]" (§3.1) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-4-1) | MUST | has no test and no annotation | "The Software Version Capability MUST only be used for displaying the version of a BGP speaker's router daemon to make troubleshooting easier" (§4) |
| DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY | [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2`](draft-abraitis-bgp-version-capability/index.md#draft-abraitis-bgp-version-capability-4-2) | MUST | has no test and no annotation | "Enabling (i.e., turning on) this capability requires bouncing all existing BGP sessions and the feature MUST be explicitly configured before an implementation advertizes the Software Version Capability" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-1`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-3-1) | MUST | has no test and no annotation | "If an implementation intends to send a single IPv6 Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 16 and include only the IPv6 Link-Local address in the Next Hop field" (§3) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-3-2`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-3-2) | MUST | has no test and no annotation | "If an implementation intends to send both a IPv6 Global and Link-Local forwarding address in the Next Hop field of the MP_REACH_NLRI, it MUST set the length of the Next Hop field to 32 and include both the IPv6 Global and Link-Local addresses in the Next Hop field" (§3) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-1`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-4-1) | MUST | has no test and no annotation | "If, after completing these procedures, there are no IPv6 next hop addresses included in the next hop, the BGP route MUST not be advertised to its peer" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-2`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-4-2) | MUST NOT | has no test and no annotation | "If the internal peer is more than one IP hop away, the BGP speaker MUST NOT include a Link-Local IPv6 next hop" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-4-3) | MUST | has no test and no annotation | "If the route is directly connected to the speaker, or if the interface address of the router through which the announced network is reachable for the speaker is the internal peer's address, the next hop MUST include its own Link-Local IPv6 address" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-4`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-4-4) | MUST NOT | has no test and no annotation | "If, after evaluating the above procedures, there are no IPv6 next hops included with the route, the route MUST NOT be announced to the remote BGP speaker" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-5`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-4-5) | MUST NOT | has no test and no annotation | "A Route Reflector (RR) reflecting a route with a link-local-only next hop MUST NOT advertise that route to a client unless the client shares the same link-layer segment as the original advertiser" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-6`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-4-6) | MUST | has no test and no annotation | "For all other clients, the RR MUST either rewrite the next hop to its own address (next-hop-self) or consider the route ineligible for advertisement to that specific peer" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-7`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-4-7) | MUST NOT | has no test and no annotation | "If no next hops are included, the route MUST NOT be announced (treat-as-withdraw)" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-8`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-4-8) | MUST NOT | has no test and no annotation | "Link-Local IPv6 next hops MUST NOT be included" for an external peer that "is multiple IP hops away from the speaker (aka \\"multihop EBGP\\")" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-9`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-4-9) | MUST NOT | has no test and no annotation | "If a Global IPv6 next hop is not included, the route MUST NOT be advertised to the external peer (treat-as-withdraw)" (§4) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-5-1`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-5-1) | MUST | has no test and no annotation | "When this combination has not been negotiated, a sender MUST follow the rules in Section 3 of [RFC8950] and encode the Next Hop as 32 octets" (§5) |
| DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY | [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1`](draft-ietf-idr-linklocal-capability/index.md#draft-ietf-idr-linklocal-capability-6-1) | MUST | has no test and no annotation | "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of \\"treat-as-withdraw\\", as described in section 7.3 of [RFC7606]" (§6) |
| RFC 3748 | [`RFC3748-2-3`](rfc3748/index.md#rfc3748-2-3) | MUST NOT | has no test and no annotation | The authenticator MUST NOT send a Success or Failure packet when retransmitting or when it fails to get a response from the peer (S2) |

128 further findings not shown here. The whole list is in data/rfc-compliance.json, and each one is on its own RFC's page under the requirement it names.

## Enrolled RFCs

| RFC | Public status | Gated MUSTs | Declared gaps | Gated with no test |
|---|---|---:|---:|---:|
| [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY`](draft-abraitis-bgp-version-capability/index.md) Software Version Capability for BGP | Supported | 11 | 0 | 11 |
| [`DRAFT-ABRAITIS-IDR-ADDPATH-PATHS-LIMIT`](draft-abraitis-idr-addpath-paths-limit/index.md) Scalability Considerations for ADD-PATH with PATHS-LIMIT | Supported | 4 | 0 | 0 |
| [`DRAFT-IETF-BESS-MUP-SAFI`](draft-ietf-bess-mup-safi/index.md) BGP Extensions for the Mobile User Plane (MUP) SAFI | Partial | 40 | 37 | 0 |
| [`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY`](draft-ietf-idr-linklocal-capability/index.md) Link-Local Next Hop Capability for BGP | Partial | 13 | 0 | 13 |
| [`DRAFT-IETF-SIDROPS-ASPA-VERIFICATION`](draft-ietf-sidrops-aspa-verification/index.md) Verification of AS_PATH Using the Resource Certificate PKI and Autonomous System Provider Authorization | Partial | 8 | 2 | 0 |
| [`DRAFT-WALTON-BGP-HOSTNAME-CAPABILITY`](draft-walton-bgp-hostname-capability/index.md) Hostname Capability for BGP | Supported | 0 | 0 | 0 |
| [`RFC 1071`](rfc1071/index.md) Computing the Internet Checksum | No public row declared | 8 | 0 | 0 |
| [`RFC 1195`](rfc1195/index.md) Use of OSI IS-IS for Routing in TCP/IP and Dual Environments | Experimental | 10 | 0 | 0 |
| [`RFC 1332`](rfc1332/index.md) The PPP Internet Protocol Control Protocol (IPCP) | Partial | 6 | 1 | 0 |
| [`RFC 1334`](rfc1334/index.md) PPP Authentication Protocols | Partial | 7 | 0 | 0 |
| [`RFC 1350`](rfc1350/index.md) The TFTP Protocol (Revision 2) | Supported | 11 | 1 | 0 |
| [`RFC 1661`](rfc1661/index.md) The Point-to-Point Protocol (PPP) | Partial | 66 | 24 | 0 |
| [`RFC 1877`](rfc1877/index.md) PPP Internet Protocol Control Protocol Extensions for Name Server Addresses | Partial | 4 | 1 | 0 |
| [`RFC 1994`](rfc1994/index.md) PPP Challenge Handshake Authentication Protocol (CHAP) | Partial | 17 | 3 | 0 |
| [`RFC 1997`](rfc1997/index.md) BGP Communities Attribute | Supported | 5 | 0 | 0 |
| [`RFC 2003`](rfc2003/index.md) IP Encapsulation within IP | No public row declared | 13 | 0 | 0 |
| [`RFC 2131`](rfc2131/index.md) Dynamic Host Configuration Protocol | Partial | 64 | 18 | 0 |
| [`RFC 2132`](rfc2132/index.md) DHCP Options and BOOTP Vendor Extensions | Partial | 34 | 1 | 0 |
| [`RFC 2181`](rfc2181/index.md) Clarifications to the DNS Specification | Partial | 23 | 1 | 0 |
| [`RFC 2205`](rfc2205/index.md) Resource ReSerVation Protocol (RSVP) -- Version 1 Functional Specification | Experimental | 6 | 2 | 0 |
| [`RFC 2328`](rfc2328/index.md) OSPF Version 2 | Partial | 25 | 1 | 0 |
| [`RFC 2347`](rfc2347/index.md) TFTP Option Extension | Supported | 4 | 0 | 0 |
| [`RFC 2348`](rfc2348/index.md) TFTP Blocksize Option | No public row declared | 5 | 0 | 0 |
| [`RFC 2349`](rfc2349/index.md) TFTP Timeout Interval and Transfer Size Options | No public row declared | 4 | 0 | 0 |
| [`RFC 2385`](rfc2385/index.md) Protection of BGP Sessions via the TCP MD5 Signature Option | Supported on Linux; FreeBSD needs a `setkey(8)` SAD entry | 9 | 0 | 0 |
| [`RFC 2473`](rfc2473/index.md) Generic Packet Tunneling in IPv6 Specification | No public row declared | 11 | 0 | 0 |
| [`RFC 2516`](rfc2516/index.md) A Method for Transmitting PPP Over Ethernet (PPPoE) | Partial | 21 | 1 | 0 |
| [`RFC 2545`](rfc2545/index.md) Use of BGP-4 Multiprotocol Extensions for IPv6 Inter-Domain Routing | Supported | 4 | 0 | 0 |
| [`RFC 2661`](rfc2661/index.md) Layer Two Tunneling Protocol "L2TP" | Partial | 18 | 1 | 0 |
| [`RFC 2759`](rfc2759/index.md) Microsoft PPP CHAP Extensions, Version 2 | Supported | 12 | 0 | 0 |
| [`RFC 2782`](rfc2782/index.md) A DNS RR for specifying the location of services (DNS SRV) | No public row declared | 7 | 0 | 0 |
| [`RFC 2784`](rfc2784/index.md) Generic Routing Encapsulation (GRE) | No public row declared | 11 | 0 | 0 |
| [`RFC 2865`](rfc2865/index.md) Remote Authentication Dial In User Service (RADIUS) | Supported for subscriber access | 29 | 0 | 0 |
| [`RFC 2866`](rfc2866/index.md) RADIUS Accounting | Supported for subscriber access | 16 | 0 | 0 |
| [`RFC 2869`](rfc2869/index.md) RADIUS Extensions | Supported for subscriber access | 11 | 0 | 0 |
| [`RFC 2890`](rfc2890/index.md) Key and Sequence Number Extensions to GRE | No public row declared | 7 | 0 | 0 |
| [`RFC 2918`](rfc2918/index.md) Route Refresh Capability for BGP-4 | Supported | 6 | 0 | 0 |
| [`RFC 2966`](rfc2966/index.md) Domain-wide Prefix Distribution with Two-Level IS-IS | Experimental | 4 | 0 | 0 |
| [`RFC 3031`](rfc3031/index.md) Multiprotocol Label Switching Architecture | No public row declared | 7 | 0 | 0 |
| [`RFC 3032`](rfc3032/index.md) MPLS Label Stack Encoding | Supported as dependency | 17 | 0 | 0 |
| [`RFC 3101`](rfc3101/index.md) The OSPF Not-So-Stubby Area (NSSA) Option | Experimental | 16 | 0 | 0 |
| [`RFC 3209`](rfc3209/index.md) RSVP-TE: Extensions to RSVP for LSP Tunnels | Experimental | 13 | 2 | 0 |
| [`RFC 3623`](rfc3623/index.md) Graceful OSPF Restart | Experimental | 13 | 2 | 0 |
| [`RFC 3630`](rfc3630/index.md) Traffic Engineering (TE) Extensions to OSPF Version 2 | Experimental | 5 | 0 | 0 |
| [`RFC 3748`](rfc3748/index.md) Extensible Authentication Protocol (EAP) | Supported in IPsec | 61 | 0 | 15 |
| [`RFC 3765`](rfc3765/index.md) NOPEER Community for Border Gateway Protocol (BGP) Route Scope Control | Supported | 0 | 0 | 0 |
| [`RFC 3768`](rfc3768/index.md) Virtual Router Redundancy Protocol (VRRP) | Experimental | 39 | 0 | 0 |
| [`RFC 3786`](rfc3786/index.md) Extending the Number of Intermediate System to Intermediate System (IS-IS) Link State PDU (LSP) Fragments Beyond the 256 Limit | No public row declared | 4 | 0 | 0 |
| [`RFC 3787`](rfc3787/index.md) Recommendations for Interoperable IP Networks using Intermediate System to Intermediate System (IS-IS) | Partial | 3 | 1 | 0 |
| [`RFC 3948`](rfc3948/index.md) UDP Encapsulation of IPsec ESP Packets | Partial | 14 | 1 | 0 |
| [`RFC 3954`](rfc3954/index.md) Cisco Systems NetFlow Services Export Version 9 | Experimental | 9 | 1 | 0 |
| [`RFC 4035`](rfc4035/index.md) Protocol Modifications for the DNS Security Extensions | Partial | 108 | 3 | 0 |
| [`RFC 4090`](rfc4090/index.md) Fast Reroute Extensions to RSVP-TE for LSP Tunnels | Experimental | 12 | 0 | 0 |
| [`RFC 4213`](rfc4213/index.md) Basic Transition Mechanisms for IPv6 Hosts and Routers | No public row declared | 23 | 0 | 0 |
| [`RFC 4271`](rfc4271/index.md) A Border Gateway Protocol 4 (BGP-4) | Partial | 100 | 15 | 0 |
| [`RFC 4301`](rfc4301/index.md) Security Architecture for the Internet Protocol | Partial | 18 | 1 | 0 |
| [`RFC 4302`](rfc4302/index.md) IP Authentication Header | Supported in OSPFv3 manual IPsec path | 34 | 0 | 34 |
| [`RFC 4303`](rfc4303/index.md) IP Encapsulating Security Payload (ESP) | Supported | 22 | 0 | 0 |
| [`RFC 4360`](rfc4360/index.md) BGP Extended Communities Attribute | Supported | 6 | 0 | 0 |
| [`RFC 4364`](rfc4364/index.md) BGP/MPLS IP Virtual Private Networks (VPNs) | Supported | 8 | 0 | 0 |
| [`RFC 4456`](rfc4456/index.md) BGP Route Reflection: An Alternative to Full Mesh Internal BGP (IBGP) | Supported | 6 | 0 | 0 |
| [`RFC 4486`](rfc4486/index.md) Subcodes for BGP Cease NOTIFICATION Message | Supported | 1 | 0 | 0 |
| [`RFC 4552`](rfc4552/index.md) Authentication/Confidentiality for OSPFv3 | Experimental | 27 | 5 | 0 |
| [`RFC 4555`](rfc4555/index.md) IKEv2 Mobility and Multihoming Protocol (MOBIKE) | Unsupported | 6 | 0 | 0 |
| [`RFC 4576`](rfc4576/index.md) Using a Link State Advertisement (LSA) Options Bit to Prevent Looping in BGP/MPLS IP Virtual Private Networks (VPNs) | No public row declared | 4 | 0 | 0 |
| [`RFC 4577`](rfc4577/index.md) OSPF as the Provider/Customer Edge Protocol for BGP/MPLS IP Virtual Private Networks (VPNs) | Not supported | 36 | 20 | 0 |
| [`RFC 4578`](rfc4578/index.md) Dynamic Host Configuration Protocol (DHCP) Options for the Intel Preboot eXecution Environment (PXE) | Supported | 5 | 0 | 0 |
| [`RFC 4659`](rfc4659/index.md) BGP-MPLS IP Virtual Private Network (VPN) Extension for IPv6 VPN | Partial | 16 | 3 | 0 |
| [`RFC 4684`](rfc4684/index.md) Constrained Route Distribution for Border Gateway Protocol/MultiProtocol Label Switching (BGP/MPLS) Internet Protocol (IP) Virtual Private Networks (VPNs) | Partial | 4 | 4 | 0 |
| [`RFC 4724`](rfc4724/index.md) Graceful Restart Mechanism for BGP | Partial | 26 | 8 | 0 |
| [`RFC 4760`](rfc4760/index.md) Multiprotocol Extensions for BGP-4 | Supported | 6 | 0 | 0 |
| [`RFC 4761`](rfc4761/index.md) Virtual Private LAN Service (VPLS) Using BGP for Auto-Discovery and Signaling | Supported | 18 | 0 | 0 |
| [`RFC 4862`](rfc4862/index.md) IPv6 Stateless Address Autoconfiguration | No public row declared | 16 | 0 | 0 |
| [`RFC 5036`](rfc5036/index.md) LDP Specification | Experimental | 14 | 7 | 0 |
| [`RFC 5072`](rfc5072/index.md) IP Version 6 over PPP | Partial | 15 | 6 | 0 |
| [`RFC 5082`](rfc5082/index.md) The Generalized TTL Security Mechanism (GTSM) | Supported on Linux | 4 | 0 | 1 |
| [`RFC 5176`](rfc5176/index.md) Dynamic Authorization Extensions to Remote Authentication Dial In User Service (RADIUS) | Supported for subscriber access | 22 | 0 | 0 |
| [`RFC 5187`](rfc5187/index.md) OSPFv3 Graceful Restart | Experimental | 4 | 0 | 0 |
| [`RFC 5216`](rfc5216/index.md) The EAP-TLS Authentication Protocol | Partial | 21 | 3 | 0 |
| [`RFC 5250`](rfc5250/index.md) The OSPF Opaque LSA Option | Experimental | 9 | 0 | 0 |
| [`RFC 5282`](rfc5282/index.md) Using Authenticated Encryption Algorithms with the Encrypted Payload of the Internet Key Exchange version 2 (IKEv2) Protocol | Supported | 19 | 0 | 18 |
| [`RFC 5286`](rfc5286/index.md) Basic Specification for IP Fast Reroute: Loop-Free Alternates | Experimental | 6 | 2 | 0 |
| [`RFC 5301`](rfc5301/index.md) Dynamic Hostname Exchange Mechanism for IS-IS | Supported | 7 | 0 | 0 |
| [`RFC 5303`](rfc5303/index.md) Three-Way Handshake for IS-IS Point-to-Point Adjacencies | Experimental | 18 | 7 | 0 |
| [`RFC 5304`](rfc5304/index.md) IS-IS Cryptographic Authentication | Experimental | 9 | 0 | 0 |
| [`RFC 5305`](rfc5305/index.md) IS-IS Extensions for Traffic Engineering | Experimental | 8 | 1 | 0 |
| [`RFC 5308`](rfc5308/index.md) Routing IPv6 with IS-IS | Experimental | 7 | 0 | 0 |
| [`RFC 5310`](rfc5310/index.md) IS-IS Generic Cryptographic Authentication | Experimental | 9 | 0 | 0 |
| [`RFC 5340`](rfc5340/index.md) OSPF for IPv6 | Partial | 23 | 5 | 0 |
| [`RFC 5392`](rfc5392/index.md) OSPF Extensions in Support of Inter-Autonomous System (AS) MPLS and GMPLS Traffic Engineering | Experimental | 12 | 4 | 0 |
| [`RFC 5443`](rfc5443/index.md) LDP IGP Synchronization | Experimental | 8 | 1 | 0 |
| [`RFC 5492`](rfc5492/index.md) Capabilities Advertisement with BGP-4 | Supported | 9 | 0 | 0 |
| [`RFC 5549`](rfc5549/index.md) Advertising IPv4 Network Layer Reachability Information with an IPv6 Next Hop | Supported | 6 | 0 | 0 |
| [`RFC 5561`](rfc5561/index.md) LDP Capabilities | No public row declared | 5 | 0 | 0 |
| [`RFC 5575`](rfc5575/index.md) Dissemination of Flow Specification Rules | Partial | 12 | 4 | 0 |
| [`RFC 5701`](rfc5701/index.md) IPv6 Address Specific BGP Extended Community Attribute | Partial | 4 | 1 | 0 |
| [`RFC 5709`](rfc5709/index.md) OSPFv2 HMAC-SHA Cryptographic Authentication | Experimental | 15 | 0 | 0 |
| [`RFC 5798`](rfc5798/index.md) Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6 | Supported | 55 | 0 | 55 |
| [`RFC 5838`](rfc5838/index.md) Support of Address Families in OSPFv3 | Experimental | 16 | 8 | 0 |
| [`RFC 5880`](rfc5880/index.md) Bidirectional Forwarding Detection (BFD) | Partial | 96 | 14 | 0 |
| [`RFC 5881`](rfc5881/index.md) Bidirectional Forwarding Detection (BFD) for IPv4 and IPv6 (Single Hop) | Partial | 23 | 6 | 0 |
| [`RFC 5882`](rfc5882/index.md) Generic Application of Bidirectional Forwarding Detection (BFD) | Partial | 3 | 0 | 0 |
| [`RFC 5883`](rfc5883/index.md) Bidirectional Forwarding Detection (BFD) for Multihop Paths | Partial | 8 | 2 | 0 |
| [`RFC 6071`](rfc6071/index.md) IP Security (IPsec) and Internet Key Exchange (IKE) Document Roadmap | No public row declared | 8 | 0 | 0 |
| [`RFC 6138`](rfc6138/index.md) LDP IGP Synchronization for Broadcast Networks | No public row declared | 2 | 0 | 0 |
| [`RFC 6286`](rfc6286/index.md) Autonomous-System-Wide Unique BGP Identifier for BGP-4 | Supported | 4 | 0 | 0 |
| [`RFC 6396`](rfc6396/index.md) Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format | Supported | 13 | 1 | 0 |
| [`RFC 6397`](rfc6397/index.md) Multi-Threaded Routing Toolkit (MRT) Border Gateway Protocol (BGP) Routing Information Export Format with Geo-Location Extensions | No public row declared | 4 | 0 | 0 |
| [`RFC 6482`](rfc6482/index.md) A Profile for Route Origin Authorizations (ROAs) | No public row declared | 9 | 0 | 0 |
| [`RFC 6549`](rfc6549/index.md) OSPFv2 Multi-Instance Extensions | No public row declared | 1 | 0 | 0 |
| [`RFC 6608`](rfc6608/index.md) Subcodes for BGP Finite State Machine Error | Partial | 3 | 3 | 0 |
| [`RFC 6793`](rfc6793/index.md) BGP Support for Four-Octet Autonomous System (AS) Number Space | Partial | 30 | 12 | 0 |
| [`RFC 6810`](rfc6810/index.md) The Resource Public Key Infrastructure (RPKI) to Router Protocol | Partial | 39 | 4 | 0 |
| [`RFC 6811`](rfc6811/index.md) BGP Prefix Origin Validation | Supported | 5 | 0 | 0 |
| [`RFC 6996`](rfc6996/index.md) Autonomous System (AS) Reservation for Private Use | No public row declared | 1 | 0 | 0 |
| [`RFC 7011`](rfc7011/index.md) Specification of the IP Flow Information Export (IPFIX) Protocol for the Exchange of Flow Information | Experimental | 19 | 1 | 0 |
| [`RFC 7012`](rfc7012/index.md) Information Model for IP Flow Information Export (IPFIX) | No public row declared | 11 | 0 | 0 |
| [`RFC 7166`](rfc7166/index.md) Supporting Authentication Trailer for OSPFv3 | Unsupported | 17 | 17 | 0 |
| [`RFC 7296`](rfc7296/index.md) Internet Key Exchange Protocol Version 2 (IKEv2) | Partial | 222 | 0 | 0 |
| [`RFC 7311`](rfc7311/index.md) The Accumulated IGP Metric Attribute for BGP | Partial | 4 | 1 | 0 |
| [`RFC 7313`](rfc7313/index.md) Enhanced Route Refresh Capability for BGP-4 | Supported | 10 | 4 | 0 |
| [`RFC 7427`](rfc7427/index.md) Signature Authentication in the Internet Key Exchange Version 2 (IKEv2) | No public row declared | 3 | 0 | 0 |
| [`RFC 7432`](rfc7432/index.md) BGP MPLS-Based Ethernet VPN | Partial | 82 | 15 | 0 |
| [`RFC 7440`](rfc7440/index.md) TFTP Windowsize Option | No public row declared | 9 | 0 | 0 |
| [`RFC 7474`](rfc7474/index.md) Security Extensions for OSPFv2 when Using Manual Key Management | Experimental | 10 | 0 | 0 |
| [`RFC 7534`](rfc7534/index.md) AS112 Nameserver Operations | Supported | 3 | 0 | 0 |
| [`RFC 7535`](rfc7535/index.md) AS112 Redirection Using DNAME | Supported | 1 | 0 | 0 |
| [`RFC 7606`](rfc7606/index.md) Revised Error Handling for BGP UPDATE Messages | Partial | 52 | 1 | 0 |
| [`RFC 7607`](rfc7607/index.md) Codification of AS 0 Processing | Supported | 5 | 0 | 0 |
| [`RFC 7611`](rfc7611/index.md) BGP ACCEPT_OWN Community Attribute | No public row declared | 5 | 0 | 0 |
| [`RFC 7684`](rfc7684/index.md) OSPFv2 Prefix/Link Attribute Advertisement | Experimental | 8 | 0 | 0 |
| [`RFC 7752`](rfc7752/index.md) North-Bound Distribution of Link-State and Traffic Engineering (TE) Information Using BGP | Partial | 26 | 4 | 0 |
| [`RFC 7770`](rfc7770/index.md) Extensions to OSPF for Advertising Optional Router Capabilities | Experimental | 11 | 0 | 0 |
| [`RFC 7854`](rfc7854/index.md) BGP Monitoring Protocol (BMP) | Partial | 13 | 0 | 0 |
| [`RFC 7858`](rfc7858/index.md) Specification for DNS over Transport Layer Security (TLS) | Partial | 19 | 1 | 0 |
| [`RFC 7871`](rfc7871/index.md) Client Subnet in DNS Queries | Partial | 38 | 6 | 0 |
| [`RFC 7911`](rfc7911/index.md) Advertisement of Multiple Paths in BGP | Supported | 9 | 0 | 0 |
| [`RFC 792`](rfc792/index.md) Internet Control Message Protocol | No public row declared | 6 | 0 | 0 |
| [`RFC 7947`](rfc7947/index.md) Internet Exchange BGP Route Server | Supported | 3 | 0 | 0 |
| [`RFC 7950`](rfc7950/index.md) The YANG 1.1 Data Modeling Language | No public row declared | 9 | 0 | 0 |
| [`RFC 7999`](rfc7999/index.md) BLACKHOLE Community | Partial | 4 | 0 | 0 |
| [`RFC 8050`](rfc8050/index.md) Multi-Threaded Routing Toolkit (MRT) Routing Information Export Format with BGP Additional Path Extensions | Partial | 6 | 1 | 0 |
| [`RFC 8092`](rfc8092/index.md) BGP Large Communities Attribute | Supported | 7 | 0 | 0 |
| [`RFC 8097`](rfc8097/index.md) BGP Prefix Origin Validation State Extended Community | No public row declared | 5 | 0 | 0 |
| [`RFC 8203`](rfc8203/index.md) BGP Administrative Shutdown Communication | Supported | 5 | 0 | 0 |
| [`RFC 8210`](rfc8210/index.md) The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 1 | Partial | 56 | 12 | 0 |
| [`RFC 8277`](rfc8277/index.md) Using BGP to Bind MPLS Labels to Address Prefixes | Partial | 34 | 10 | 0 |
| [`RFC 8414`](rfc8414/index.md) OAuth 2.0 Authorization Server Metadata | Partial | 7 | 1 | 0 |
| [`RFC 8484`](rfc8484/index.md) DNS Queries over HTTPS (DoH) | Partial | 16 | 1 | 0 |
| [`RFC 8571`](rfc8571/index.md) BGP - Link State (BGP-LS) Advertisement of IGP Traffic Engineering Performance Metric Extensions | No public row declared | 4 | 0 | 0 |
| [`RFC 8654`](rfc8654/index.md) Extended Message Support for BGP | Supported | 12 | 0 | 0 |
| [`RFC 8665`](rfc8665/index.md) OSPF Extensions for Segment Routing | Partial | 47 | 14 | 0 |
| [`RFC 8666`](rfc8666/index.md) OSPFv3 Extensions for Segment Routing | Partial | 31 | 3 | 0 |
| [`RFC 8669`](rfc8669/index.md) Segment Routing Prefix Segment Identifier Extensions for BGP | Partial | 25 | 10 | 0 |
| [`RFC 8671`](rfc8671/index.md) Support for Adj-RIB-Out in the BGP Monitoring Protocol (BMP) | Supported within BMP sender scope | 10 | 1 | 1 |
| [`RFC 8707`](rfc8707/index.md) Resource Indicators for OAuth 2.0 | No public row declared | 7 | 0 | 0 |
| [`RFC 8907`](rfc8907/index.md) The Terminal Access Controller Access-Control System Plus (TACACS+) Protocol | Partial | 12 | 2 | 0 |
| [`RFC 8950`](rfc8950/index.md) Advertising IPv4 Network Layer Reachability Information (NLRI) with an IPv6 Next Hop | Supported | 6 | 0 | 0 |
| [`RFC 8955`](rfc8955/index.md) Dissemination of Flow Specification Rules | Partial | 22 | 5 | 0 |
| [`RFC 8956`](rfc8956/index.md) Dissemination of Flow Specification Rules for IPv6 | Partial | 9 | 7 | 0 |
| [`RFC 9003`](rfc9003/index.md) Extended BGP Administrative Shutdown Communication | Supported | 4 | 0 | 0 |
| [`RFC 9012`](rfc9012/index.md) The BGP Tunnel Encapsulation Attribute | Partial | 75 | 51 | 0 |
| [`RFC 905`](rfc905/index.md) ISO Transport Protocol Specification (ISO DP 8073) | No public row declared | 9 | 0 | 0 |
| [`RFC 9069`](rfc9069/index.md) Support for Local RIB in the BGP Monitoring Protocol (BMP) | Supported | 15 | 0 | 0 |
| [`RFC 9072`](rfc9072/index.md) Extended Optional Parameters Length for BGP OPEN Message | Partial | 9 | 4 | 0 |
| [`RFC 9085`](rfc9085/index.md) Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing | Partial | 12 | 9 | 0 |
| [`RFC 9086`](rfc9086/index.md) Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing BGP Egress Peer Engineering | Partial | 12 | 10 | 0 |
| [`RFC 9136`](rfc9136/index.md) IP Prefix Advertisement in Ethernet VPN (EVPN) | Partial | 14 | 5 | 0 |
| [`RFC 9234`](rfc9234/index.md) Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages | Supported | 19 | 0 | 0 |
| [`RFC 9252`](rfc9252/index.md) BGP Overlay Services Based on Segment Routing over IPv6 (SRv6) | Partial | 19 | 8 | 0 |
| [`RFC 9256`](rfc9256/index.md) Segment Routing Policy Architecture | Partial | 22 | 6 | 0 |
| [`RFC 9319`](rfc9319/index.md) The Use of maxLength in the Resource Public Key Infrastructure (RPKI) | No public row declared | 4 | 0 | 0 |
| [`RFC 9494`](rfc9494/index.md) Long-Lived Graceful Restart for BGP | Partial | 25 | 5 | 0 |
| [`RFC 9514`](rfc9514/index.md) Border Gateway Protocol - Link State (BGP-LS) Extensions for Segment Routing over IPv6 (SRv6) | Partial | 13 | 13 | 0 |
| [`RFC 9552`](rfc9552/index.md) Distribution of Link-State and Traffic Engineering Information Using BGP | Partial | 48 | 1 | 0 |
| [`RFC 9568`](rfc9568/index.md) Virtual Router Redundancy Protocol (VRRP) Version 3 for IPv4 and IPv6 | Partial | 59 | 2 | 0 |
| [`RFC 9582`](rfc9582/index.md) The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2 | Unsupported | 10 | 0 | 0 |
| [`RFC 9687`](rfc9687/index.md) Border Gateway Protocol 4 (BGP-4) Send Hold Timer | Supported | 13 | 0 | 0 |
| [`RFC 9728`](rfc9728/index.md) OAuth 2.0 Protected Resource Metadata | No public row declared | 7 | 0 | 0 |
| [`RFC 9830`](rfc9830/index.md) BGP Extensions for the Advertisement of Segment Routing (SR) Policies | Partial | 96 | 20 | 0 |
| [`SFLOW-V5`](sflow-v5/index.md) sFlow: A Method for Monitoring Traffic in Switched and Routed Networks | Experimental | 16 | 3 | 0 |

## Summaries that are not enrolled

- `backlog`: the requirements have not been extracted from the document yet; this is work owed rather than a decision
- `blocked`: something outside the summary stops the extraction, and it is named in the reason
- `non-normative`: the document imposes no MUST-level obligation on an implementation, so there is nothing to gate
- `out-of-scope`: the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap

| RFC | Disposition | Reason |
|---|---|---|
| [`DRAFT-IETF-SIDROPS-8210BIS`](draft-ietf-sidrops-8210bis/index.md) The Resource Public Key Infrastructure (RPKI) to Router Protocol, Version 2 | backlog | The RPKI to Router Protocol, Version 2. Split out of rfc9582 on 2026-09-01 because the obligations below are stated by this draft and not by RFC 9582, which profiles the ROA certificate. It is not enrolled because its obligations are not yet proven and the draft is still in the RFC Editor queue, so a version bump can restate them. |
| [`RFC 1035`](rfc1035/index.md) Domain Names - Implementation and Specification | backlog | Domain Names: Implementation and Specification. Re-authored 2026-07-30 and it now declares 27 MUST-level obligations read from the indicative prose of a 1987 document (0 capitalised keywords, 23 lowercase must), so this is no longer an empty checklist. It is not enrolled because the obligations are not all proven and the unproven ones need an owner ruling, not an implementer's annotation. The obligation with no code path in Ze is zone transfer: Ze performs none, and the owner ruled RFC 1035 out of scope on 2026-08-18, so that work is not to be started. The 512-octet UDP bound and the TC bit ARE enforced -- send calls Msg.Truncate(udpReplyLimit(r)) for a datagram reply in internal/core/dnsserver/handler.go, and udpReplyLimit holds the Section 2.3.4 floor while letting an RFC 6891 Section 6.2.3 OPT record raise it. An unsupported inverse query DOES draw Not Implemented: Authoritative branches on the opcode before any zone lookup, in the same file. It was the one obligation the 73-section walk found OUTSIDE the summary's declared scope and added to it. The response TTL is deliberately not raised to the SOA MINIMUM -- RFC 2308 Section 4 withdrew that rule, hdr in internal/plugins/geodns/server.go applies no floor, and TestRFC2308_NoZoneWideTTLFloor holds the decision. About 6 requirements admit only a positive polarity because miekg/dns owns the wire codec and no Ze-side change can break them. Escalated for scoping per OR-1b. |
| [`RFC 4762`](rfc4762/index.md) Virtual Private LAN Service (VPLS) Using Label Distribution Protocol (LDP) Signaling | blocked | VPLS using LDP signaling. Written 2026-09-01 so the public row this RFC has always carried is declared by a summary rather than authored on a page nobody could tie back to a document. It is not enrolled because there is no source text at rfc/full/rfc4762.txt: fetch https://www.rfc-editor.org/rfc/rfc4762.txt, then extract. Ze speaks the BGP-signalled VPLS of RFC 4761 and not this one, so the row claims Unsupported and no obligation here is gated. |
| [`RFC 5065`](rfc5065/index.md) Autonomous System Confederations for BGP | blocked | BGP confederations. Written 2026-09-01 for the same reason as the other six rows that had no summary: the public claim now lives in the document it is about. It is not enrolled because there is no source text at rfc/full/rfc5065.txt: fetch https://www.rfc-editor.org/rfc/rfc5065.txt, then extract. The row claims Unsupported, so no obligation here is gated. |
| [`RFC 5120`](rfc5120/index.md) M-ISIS: Multi Topology (MT) Routing in Intermediate System to Intermediate Systems (IS-ISs) | blocked | IS-IS multi-topology routing. Written 2026-09-01 so the public row is declared by a summary. It is not enrolled because there is no source text at rfc/full/rfc5120.txt: fetch https://www.rfc-editor.org/rfc/rfc5120.txt, then extract. Ze runs IS-IS single-topology dual-stack only and the row claims Unsupported, so no obligation here is gated. |
| [`RFC 5925`](rfc5925/index.md) The TCP Authentication Option | blocked | The TCP Authentication Option. Written 2026-09-01 so the public row is declared by a summary. It is not enrolled because there is no source text at rfc/full/rfc5925.txt: fetch https://www.rfc-editor.org/rfc/rfc5925.txt, then extract. Ze implements the RFC 2385 TCP MD5 signature option and not TCP-AO, and the row claims Unsupported, so no obligation here is gated. |
| [`RFC 6514`](rfc6514/index.md) BGP Encodings and Procedures for Multicast in MPLS/BGP IP VPNs | out-of-scope | BGP Encodings and Procedures for Multicast in MPLS/BGP IP VPNs. OUT OF SCOPE by owner decision, 2026-09-01, marked for future development. The extraction is COMPLETE: the source text is at rfc/full/rfc6514.txt and this summary declares all 201 requirements, 133 of them MUST-level, so a later decision to build MVPN starts from the obligations rather than from nothing. What Ze has today is NLRI plumbing and nothing the RFC is about: the Section 4 route-type split (splitMVPN, internal/core/bgp/nlri/nlrisplit/mvpn.go), an NLRI codec, a config route parser for three of the seven route types, and opaque Adj-RIB-In storage. Exactly one MUST-level requirement is met and it is met vacuously -- RFC6514-9.1.1-10 says the Leaf Information Required flag "MUST be set to zero and MUST be ignored on receipt", and Ze ignores it by never parsing a PMSI byte, because knownAttrParsers leaves attribute code 22 nil. Absent entirely: the PMSI Tunnel attribute, the PE Distinguisher Labels attribute (code 27 has no constant), the Source AS and VRF Route Import extended communities, auto-discovery, the C-multicast route exchange, S-PMSI routes, inter-AS and ASBR operation, upstream multicast hop selection (SAFI 129 is unregistered), and every protocol the document leans on -- PIM, mLDP, RSVP-TE P2MP and MSDP. No {gap} annotation is written for any of it: a gap is an ISSUE and this is a DECISION (ai/rules/rfc-compliance.md), and 132 gap rows would record a feature nobody chose to build as 132 conformance failures. |
| [`RFC 6987`](rfc6987/index.md) OSPF Stub Router Advertisement | backlog | OSPF Stub Router Advertisement. Its five obligations sit behind bare [LSA]-style category tags, which parse_checklist_line reads as prose rather than as requirements, so the summary captures zero at any level while the source is normative. Declared backlog and not non-normative on purpose: calling it non-normative would launder five unparsed obligations into a decision, which is exactly the shape D5 exists to expose. |
| [`RFC 7454`](rfc7454/index.md) BGP Operations and Security | non-normative | BGP Operations and Security, published as BCP 194 with IETF category Best Current Practice. A capitalised MUST / MUST NOT / SHALL / SHALL NOT scan over rfc/full/rfc7454.txt hits four keywords and all four sit inside the RFC 2119 key-words sentence of section 1.1, which tells a reader how to read the other sentences and states no obligation of its own. Outside that sentence the document uses no MUST-level keyword except one NOT REQUIRED in section 5.1, and that phrase negates a requirement instead of stating one. The summary written 2026-08-08 therefore captures 64 requirements and gates none of them: 42 SHOULD, 12 SHOULD NOT, 8 RECOMMENDED, 1 MAY, and the single NOT REQUIRED of section 5.1 recorded at the OPTIONAL level. The document also addresses network administrators rather than protocol implementers, and section 12 states that it "does not aim to describe existing BGP implementations". A zero-MUST BCP can reach the public ledger two ways, as a non-normative disposition or as a manual-walk extraction sign-off with a register-reason, and that choice is a ledger judgement for the owner. Thomas made it on 2026-08-12: non-normative, on the grounds that the scan recorded above finds no MUST-level keyword outside the key-words sentence, so backlog overstated a debt this text does not create. The choice is no longer open. |
| [`RFC 7627`](rfc7627/index.md) Transport Layer Security (TLS) Session Hash and Extended Master Secret Extension | backlog | TLS Session Hash and Extended Master Secret Extension. Summary written 2026-08-24 against rfc/full/rfc7627.txt, which was fetched the same day. It declares 27 requirements: 17 MUST-level over 5 sections, 9 at SHOULD level (8 SHOULD / SHOULD NOT plus one NOT RECOMMENDED) and 1 MAY. It is NOT enrolled because not one of the 17 has a producer inside Ze, so neither route to enrolment is an implementer's to take: there is no Ze function a positive and a negative test could tag, and annotating the remainder is a conformance judgement ai/rules/rfc-compliance.md reserves to the owner. WHERE THE OBLIGATIONS ARE DISCHARGED. Go's crypto/tls owns the extension end to end. Its client sets extendedMasterSecret on every ClientHello it builds (makeClientHello, crypto/tls/handshake_client.go), clearing it only for an ECH inner hello; its server copies the client's bit into the ServerHello (serverHandshakeState.processClientHello, crypto/tls/handshake_server.go); the negotiated result lands on Conn.extMasterSecret; and the Section 5.3 abbreviated-handshake mismatch rules are enforced in clientHandshakeState.processServerHello and serverHandshakeState.checkForResumption. Ze holds no ClientHello or ServerHello encoder and no master-secret derivation, so RFC7627-5.1-1, 5.2-1, 5.2-2, 5.2-4, 5.2-6, 5.3-3, 5.3-4, 5.3-6, 5.3-8, 5.3-9, 5.3-10 and 6.4-2 have no Ze-side producer at all. The Section 5.4 downgrade containment (RFC7627-5.4-1 to 5.4-4 and 5.4-6) is enforced below Ze too: Conn.connectionStateLocked selects noEKMBecauseNoEMS on `c.vers != VersionTLS13 && !c.extMasterSecret` (Conn.connectionStateLocked, crypto/tls/conn.go), which is the MUST NOT of RFC7627-5.4-1 executed by the library rather than by its caller. Ze sets no tls.Config.Renegotiation and computes no tls-unique -- the sha256(TLSUnique) fallback was removed from tlsMethod.deriveMSK -- and the authenticator config sets SessionTicketsDisabled so it offers no abbreviated handshake to resume (newTLSMethod, internal/component/ike/eap/eap_tls.go). WHAT ZE ACTUALLY PRODUCES is the refusal path, and it is two functions in internal/component/ike/eap/eap_tls.go: exportEAPTLSMSK returns the crypto/tls error rather than a zero MSK, and eapTLS12ExportRefused names the peer, the negotiated version, RFC 7627 and the operator's three remedies (move the peer to TLS 1.3 per RFC 9190, add RFC 7627 to its TLS 1.2 stack, or configure another EAP method). RFC 5216 Section 2.3 defines the EAP-TLS MSK as that export, so a TLS 1.2 peer whose session carries no extended master secret cannot authenticate. strongSwan 5.9.14 reaches that state by DEFAULT rather than by limitation: charon ships version_max = 1.2 and negotiates no RFC 7627, while charon.tls.version_max = 1.3 on the same build reaches an established SA (test/interop-ipsec/scenarios/eap-tls13/strongswan.conf, and the failing counterpart is test/interop-ipsec/scenarios/eap-tls). TWO FACTS AN OWNER RULING HAS TO ABSORB. First, RFC 7627 IS OBSOLETE. RFC 9846 (The Transport Layer Security (TLS) Protocol Version 1.3, July 2026) obsoletes RFC 5077, 5246, 6961, 7627, 8422 and 8446; its Appendix D renames the extension to Extended Main Secret and the code point to extended_main_secret, and leaves the Section 4 PRF label unchanged for compatibility; its Appendix E states the only obligation it adds about the extension, a SHOULD that an implementation supporting both TLS 1.3 and earlier versions indicate the use of the Extended Main Secret extension in its APIs whenever TLS 1.3 is used. ai/rules/rfc-compliance.md says the lineage that matters runs FORWARD, so the document that states what Ze owes today is RFC 9846, and rfc/full/rfc9846.txt is not in this repository. Enrolling rfc7627 would gate a superseded text. Second, the tlsunsafeekm override is GONE from this checkout, as of the toolchain bump on 2026-08-25. go.mod pins `toolchain go1.27.0`, whose internal/godebugs/table.go carries `{Name: "tlsunsafeekm", Removed: 27, Old: one}`, so a process that sets it to its old value raises a fatal error before main() rather than getting the unsafe export. crypto/tls agrees at the other end: noEKMBecauseNoEMS (crypto/tls/prf.go) now returns the bare sentence with no override clause at all. So the claim cmd/ze/main.go and the RFC 5216 row of docs/features/rfc-status.md both make, that no override remains, is true of what this tree builds; it was false while the toolchain was 1.26. Escalated for a scoping ruling, per the route rfc1035 and rfc9190 took. |
| [`RFC 8195`](rfc8195/index.md) Use of BGP Large Communities | non-normative | Use of BGP Large Communities, IETF category Informational. A capitalised MUST / MUST NOT / SHALL / SHALL NOT / REQUIRED scan over rfc/full/rfc8195.txt hits zero keywords, and the document invokes neither RFC 2119 nor RFC 8174 nor BCP 14 anywhere, so it declares no key-words machinery for a reader to read its prose by. Its own abstract calls it examples and inspiration for operator application of BGP Large Communities. The summary written against it captures 3 requirements and gates none: RFC8195-2-1, RFC8195-2.2-1 and RFC8195-4.3.3-1, all at SHOULD. Section 3.2 and Section 4.1.1 both say an AS could assign a function number, which states a convention rather than an obligation. Thomas ruled on 2026-08-12 that this is non-normative rather than backlog. |
| [`RFC 8326`](rfc8326/index.md) Graceful BGP Session Shutdown | blocked | Graceful BGP Session Shutdown. No source text at rfc/full/rfc8326.txt or rfc/drafts/rfc8326.txt, so check_enrolment refuses the enrolment. Fetch https://www.rfc-editor.org/rfc/rfc8326.txt, then extract. |
| [`RFC 8362`](rfc8362/index.md) OSPFv3 Link State Advertisement (LSA) Extensibility | out-of-scope | OSPFv3 Link State Advertisement (LSA) Extensibility. OUT OF SCOPE as a document by owner decision, 2026-09-01, marked for future development. The extraction is COMPLETE: the source text is at rfc/full/rfc8362.txt and this summary declares all 50 requirements, 38 of them MUST-level. What Ze has is a by-product of RFC 8666 Segment Routing rather than the framework this document defines: three of the seven Extended LSA types are originated, and only when segment routing is enabled (v6OriginateSR, internal/plugins/ospf/sr_origination_v6.go); receipt decoding is reached from one place and reads Prefix-SIDs alone (v6ReceivedPrefixSIDs, sr_reception_v6.go); no SPF calculation reads an Extended LSA, because the base Router-LSA remains the sole SPF vertex; and there is no RFC 8362 configuration surface and none of its Appendix A or B migration machinery. Nine MUST-level requirements are genuinely unmet and 20 more are met only because Ze performs no action on their subject. The most serious is RFC8362-2-1: the seven LS type constants carry the U-bit clear where Section 2 requires it set, which is recorded as a defect in plan/journal/declared-format-contradicts-payload.md and is a fix rather than a scope question. No {gap} annotation is written here, because the scope decision covers the document and the U-bit defect is tracked where a fix is owed. |
| [`RFC 8538`](rfc8538/index.md) Notification Message Support for BGP Graceful Restart | blocked | Notification message support for BGP graceful restart. Written 2026-09-01 so the public row is declared by a summary. It is not enrolled because there is no source text at rfc/full/rfc8538.txt: fetch https://www.rfc-editor.org/rfc/rfc8538.txt, then extract. The row claims Unsupported, so no obligation here is gated. |
| [`RFC 9129`](rfc9129/index.md) YANG Data Model for the OSPF Protocol | blocked | YANG Data Model for the OSPF Protocol. No source text at rfc/full/rfc9129.txt or rfc/drafts/rfc9129.txt, so check_enrolment refuses the enrolment. Fetch https://www.rfc-editor.org/rfc/rfc9129.txt, then extract. |
| [`RFC 9190`](rfc9190/index.md) EAP-TLS 1.3: Using the Extensible Authentication Protocol with TLS 1.3 | backlog | EAP-TLS 1.3: Using EAP with TLS 1.3. Summary written 2026-08-01 and it declares 51 MUST-level obligations over 19 sections. It is NOT enrolled because 49 of them are not yet proven by a tagged test, and the two routes to enrolment are both closed to an implementer: proving all 51 in both polarities is spec-sized work, and annotating the remainder is a conformance judgement ai/rules/rfc-compliance.md reserves to the owner. What Ze does implement today is the Section 2.3 key derivation and the Section 2.5 protected success result indication. exportEAPTLSMSK (internal/component/ike/eap/eap_tls.go) selects the exporter label EXPORTER_EAP_TLS_Key_Material, the Type-Code context and the 128-octet length whenever the negotiated version is TLS 1.3, and test/interop-ipsec/scenarios/eap-tls13 exercises that path against strongSwan. tlsMethod.indicateSuccess (same file, added 2026-08-12) writes the encrypted TLS record carrying application data 0x00 in the round that completes the handshake, so RFC9190-2.5-1 and RFC9190-2.5-2 now carry both polarities; test/interop-ipsec/scenarios/responder-eap-tls13 proves it with Ze in the EAP-TLS SERVER role, and with the write reverted strongSwan logs missing protected success indication for EAP-TLS with TLS 1.3 and the SA never establishes. PeerSession.handleTLSRequest still does not CONSUME the indication: it answers the record with the no-data EAP-Response Section 2.5 step 4 asks for, but never decrypts it, so a Ze peer cannot tell a server that sent one from a server that did not. The published RFC states no peer-side obligation and errata 7577, which proposes one, is Reported rather than Verified. Escalated for a scoping ruling rather than decided, per the same route rfc1035 and rfc5301 took. |
| [`RFC 9384`](rfc9384/index.md) A BGP Cease NOTIFICATION Subcode for Bidirectional Forwarding Detection (BFD) | non-normative | A BGP Cease NOTIFICATION Subcode for Bidirectional Forwarding Detection (BFD), IETF category Standards Track. The document carries the RFC 2119 / RFC 8174 key-words paragraph at Section 2, and a capitalised MUST / MUST NOT / SHALL / SHALL NOT / REQUIRED scan over rfc/full/rfc9384.txt hits those five words on one line only, line 101, which is the key-words sentence itself. That sentence tells a reader how to read the other sentences and states no obligation of its own. Outside it the text uses no MUST-level keyword anywhere. The summary written 2026-09-01 therefore captures three requirements and gates none: RFC9384-3-1 at Section 3, and RFC9384-4-1 and RFC9384-4-2 at Section 4, all three at SHOULD. Section 5 says the subcode "is purely informational and has no impact on the BGP Finite State Machine beyond that already documented by [RFC4271], Sections 6.6 and 6.7", so the document adds one registry value and three recommendations about using it. A zero-MUST document can reach the public ledger two ways, as this disposition or as a manual-walk extraction sign-off with a register-reason. This disposition is the route taken, because the sign-off at rfc/extraction/rfc9384.json declares the register the source derives, prose, and the second route would need it to declare the weaker manual-walk grade instead. That sign-off bounds the three-row checklist against the source text. |
