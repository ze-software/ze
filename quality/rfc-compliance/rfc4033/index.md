# RFC 4033 - DNS Security Introduction and Requirements

No row in the public ledger. Every requirement this repository extracted from RFC 4033, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 0 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 0 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 0 | of 0 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 0 | of 0 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (non-normative), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 0 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 0 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 0 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 0 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 0 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (non-normative) |
| Requirements | 0 |
| Gated MUST-level | 0 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4033.md` |
| Requirement shard | no requirement declared, so no shard is generated |
| RFC text | `rfc/full/rfc4033.txt` |

## Enrolment

Not enrolled (non-normative, the document imposes no MUST-level obligation on an implementation, so there is nothing to gate): DNS Security Introduction and Requirements, the introduction to the DNSSEC document set. The text carries no RFC 2119 key-words section, and a capitalised MUST / MUST NOT / SHALL / SHALL NOT / REQUIRED / SHOULD / MAY scan over rfc/full/rfc4033.txt finds zero keywords. Its lowercase "must" sentences introduce obligations that RFC 4034 (the resource records) and RFC 4035 (the protocol modifications) state with RFC 2119 keywords; both carry their own summaries. Every such sentence is classified in rfc/extraction/rfc4033.json.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 4033.

## Coverage

RFC 4033 declares no MUST-level requirement, so the gate counts nothing here.

## Attainable ceiling

Mandatory ceiling: no rows. Optional ceiling: no rows. Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

RFC 4033 declares no requirement, so this summary generates no shard.

## Gaps and untested MUSTs

RFC 4033 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

RFC 4033 carries no gated, tagged or audited requirement, so there is no proof state to state.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | prose |
| Source | rfc/full/rfc4033.txt |
| Source fingerprint | 3eaa39b61a869484 |
| Record | rfc/extraction/rfc4033.json |
| Mapped sentences | 0 |
| Declined as scope | 12 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 3 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 2 | walked | not stated |
| `7` | not stated | 2 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 2 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `14.1` | not stated | 0 | walked | not stated |
| `14.2` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 4033 is the introduction to the DNSSEC set and uses no RFC 2119 keyword; this lowercase 'must' introduces an obligation on zone signers, validating resolvers or zone transfer that RFC 4034 and RFC 4035 state with RFC 2119 keywords. | Note that the resolver must always be guided by local policy when deciding whether to authenticate a new public key, even if the local policy is simply to authenticate any new public key for which the resolver is able verify the signature. |
| `3.1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 4033 is the introduction to the DNSSEC set and uses no RFC 2119 keyword; this lowercase 'must' introduces an obligation on zone signers, validating resolvers or zone transfer that RFC 4034 and RFC 4035 state with RFC 2119 keywords. | Note that the private keys used to sign zone data must be kept secure and should be stored offline when practical. |
| `3.1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 4033 is the introduction to the DNSSEC set and uses no RFC 2119 keyword; this lowercase 'must' introduces an obligation on zone signers, validating resolvers or zone transfer that RFC 4034 and RFC 4035 state with RFC 2119 keywords. | Security-aware resolvers authenticate zone information by forming an authentication chain from a newly learned public key back to a previously known authentication public key, which in turn either has been configured into the resolver or must have been learned and verified previously. |
| `3.1:3` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 4033 is the introduction to the DNSSEC set and uses no RFC 2119 keyword; this lowercase 'must' introduces an obligation on zone signers, validating resolvers or zone transfer that RFC 4034 and RFC 4035 state with RFC 2119 keywords. | Therefore, the resolver must be configured with at least one trust anchor. |
| `6:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 4033 is the introduction to the DNSSEC set and uses no RFC 2119 keyword; this lowercase 'must' introduces an obligation on zone signers, validating resolvers or zone transfer that RFC 4034 and RFC 4035 state with RFC 2119 keywords. | Security-aware resolvers must also be capable of forming an authentication chain from a newly learned zone back to an authentication key, as described above. |
| `6:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Describes the situation in which a resolver must rely on an unsigned zone; the lowercase 'must' is conditional narration, and the consequence is stated as 'will need a local policy'. | If a security-aware resolver must rely on an unsigned zone or a name server that is not security aware, the resolver may not be able to validate DNS responses and will need a local policy on whether to accept unverified responses. |
| `7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive statement that most queries come from stub resolvers; it says the protocol does not require it and obliges nobody. | Although not strictly required to do so by the protocol, most DNS queries originate from stub resolvers. |
| `7:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Introduces the stub-resolver trust obligation, which RFC 4035 Section 4.9.3 states with RFC 2119 keywords (RFC4035-4.9.3-2). | Even a security-oblivious stub resolver may benefit from DNSSEC if the recursive name servers it uses are security-aware, but for the stub resolver to place any real reliance on DNSSEC services, the stub resolver must trust both the recursive name servers in question and the communication channels between itself and those name servers. |
| `9:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Introduces the EDNS0 payload-size obligation of a security-aware name server, which RFC 4035 Section 3 states with RFC 2119 keywords as RFC4035-3-1 and RFC4035-3-2. | Because inclusion of these DNSSEC RRs could easily cause UDP message truncation and fallback to TCP, a security-aware name server must also support the EDNS "sender's UDP payload" mechanism. |
| `12:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 4033 is the introduction to the DNSSEC set and uses no RFC 2119 keyword; this lowercase 'must' introduces an obligation on zone signers, validating resolvers or zone transfer that RFC 4034 and RFC 4035 state with RFC 2119 keywords. | In order for a security-aware resolver to validate a DNS response, all zones along the path from the trusted starting point to the zone containing the response zones must be signed, and all name servers and resolvers involved in the resolution process must be security-aware, as defined in this document set. |
| `12:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | RFC 4033 is the introduction to the DNSSEC set and uses no RFC 2119 keyword; this lowercase 'must' introduces an obligation on zone signers, validating resolvers or zone transfer that RFC 4034 and RFC 4035 state with RFC 2119 keywords. | Thus, while DNSSEC can provide data origin authentication and data integrity for RRsets, it cannot do so for zones, and other mechanisms (such as TSIG, SIG(0), or IPsec) must be used to protect zone transfer operations. |
| `14.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Intellectual Property boilerplate inviting disclosure of rights; no obligation on an implementation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4033, so its obligations are stated where they were written.
