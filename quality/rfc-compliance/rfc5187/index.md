# RFC 5187 - OSPFv3 Graceful Restart

Experimental. Every requirement this repository extracted from RFC 5187, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 4 of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 66.7% | 8 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 4 | of 4 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 4 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 4 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 12 |
| Recorded audit verdicts | 4 |
| Discrimination records | 8 |
| Summary | `rfc/short/rfc5187.md` |
| Requirement shard | `rfc/requirements/rfc5187.md` |
| RFC text | `rfc/full/rfc5187.txt` |

## Enrolment

Enrolled: OSPFv3 Graceful Restart: four MUST-level requirements, all met and tested with both polarities. RFC5187-2.2-1 (Grace Period TLV always in a grace-LSA) and RFC5187-2.2-2 (Restart Reason TLV always present) via the OSPFv3 grace-LSA body codec (internal/plugins/ospf/v3/packet/lsa_grace.go, which the OSPFv3 origination v6OriginateGraceLSA in gr_preserve.go encodes and the helper's grInspectV6Update decodes): TestRFC5187OriginatedGraceLSACarriesPeriodAndReasonTLVs (the originated LSA's body is the literal Type 1 Length 4 and Type 2 Length 1 TLVs) and TestRFC5187HelperIgnoresGraceLSAMissingMandatoryTLV (a peer's grace-LSA lacking either TLV is ignored by the helper). Ze originates the OSPFv3 link-scoped grace-LSA (LS Type 0x000B). RFC5187-3.1-1 (preserve LSA-ID to prefix correspondence across restart) and RFC5187-3.2-1 (preserve OSPFv3 Interface ID across restart) via the NVS preservation maps PrefixLSIDs/InterfaceIDs (internal/plugins/ospf/gr_nvs.go:41-45): TestRestartFactPersistsAcrossRestart (maps read back intact after a restart) and TestStaleRestartFactIgnored (an expired/cleared restart fact is inactive, so stale IDs are not restored). No SHOULD/MAY requirements are gated.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Restarter and helper behavior for OSPFv3.

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC5187-2.2-1`](#rfc5187-2.2-1), [`RFC5187-2.2-2`](#rfc5187-2.2-2), [`RFC5187-3.1-1`](#rfc5187-3.1-1), [`RFC5187-3.2-1`](#rfc5187-3.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5187-2.2-1` | Grace Period (Type=1, Length=4). The number of seconds that the router's neighbors should continue to advertise the router as fully adjacent, regardless of the state of database synchronization between the router and its neighbors. This TLV MUST always appear in a grace-LSA. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestRFC5187OriginatedGraceLSACarriesPeriodAndReasonTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_grace_tlv_test.go#L30). **negative:** `unit/verify` [`TestRFC5187HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_grace_tlv_test.go#L109) |
| `RFC5187-2.2-2` | Graceful restart reason (Type=2, Length=1). Encodes the reason for the router restart, as one of the following: 0 (unknown), 1 (software restart), 2 (software reload/upgrade), or 3 (switch to redundant control processor). This TLV MUST always appear in a grace-LSA. (§2.2) | MUST | 2.2 | **positive:** `unit/verify` [`TestRFC5187OriginatedGraceLSACarriesPeriodAndReasonTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_grace_tlv_test.go#L33). **negative:** `unit/verify` [`TestRFC5187HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_grace_tlv_test.go#L114) |
| `RFC5187-3.1-1` | Hence, to avoid network churn during graceful restart, the restarting router MUST preserve the LSA ID to prefix correspondence across graceful restarts. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC5187LSAIDToPrefixPreservedAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_preserve_restart_test.go#L85). **positive:** `unit/verify` [`TestRestartFactPersistsAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_gr_nvs_test.go#L60). **negative:** `unit/verify` [`TestRFC5187LSAIDToPrefixPreservedAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_preserve_restart_test.go#L98). **negative:** `unit/verify` [`TestStaleRestartFactIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_gr_nvs_test.go#L110) |
| `RFC5187-3.2-1` | Therefore, the OSPFv3 Interface ID, as described in section 3.1.2 of [OSPFv3], MUST be preserved by the restarting router across restarts. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestRFC5187InterfaceIDPreservedAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_preserve_restart_test.go#L154). **positive:** `unit/verify` [`TestRestartFactPersistsAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_gr_nvs_test.go#L63). **negative:** `unit/verify` [`TestRFC5187InterfaceIDPreservedAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_preserve_restart_test.go#L144). **negative:** `unit/verify` [`TestStaleRestartFactIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_gr_nvs_test.go#L114) |

## Gaps and untested MUSTs

RFC 5187 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5187-2.2-1`](#rfc5187-2.2-1)

Grace Period (Type=1, Length=4). The number of seconds that the router's neighbors should continue to advertise the router as fully adjacent, regardless of the state of database synchronization between the router and its neighbors. This TLV MUST always appear in a grace-LSA. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c22 re-judge. Forbidden: a grace-LSA without the Grace Period TLV (Type=1, Length=4). Positive TestRFC5187OriginatedGraceLSACarriesPeriodAndReasonTLVs drives the OSPFv3 origination v6OriginateGraceLSA (gr_restarter.go calls it) and compares the installed LSA body after the 20-octet v3 header with literal octets 00 01 00 04 <period> for period 120 and 0, so a wrong type, wrong length or dropped TLV goes red (author overlay GraceTLVPeriod=3 red; recorded revert of v3/packet/lsa_grace.go::tlvs). Negative under OWNER RULING 2 (generate row: the negative is how Ze handles a peer PDU lacking it; RFC 5187 prescribes no handling and the helper cannot run a window it was not given, so ignore): TestRFC5187HelperIgnoresGraceLSAMissingMandatoryTLV feeds a reason-only v3 Grace-LSA to grInspectV6Update (instance.go receive path) while helping X and asserts the helper grace end is unchanged, with a both-TLV control that moves it to now+600, isolating the hasPeriod check (author overlay: removing hasPeriod reds only that subtest; recorded revert of ::decodeGraceLSA). Tags moved off the OSPFv2 RFC 3623 codec units (D-15).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5187HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_grace_tlv_test.go#L109) | unit/verify | revert, verified |
| positive | [`TestRFC5187OriginatedGraceLSACarriesPeriodAndReasonTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_grace_tlv_test.go#L30) | unit/verify | revert, verified |

### [`RFC5187-2.2-2`](#rfc5187-2.2-2)

Graceful restart reason (Type=2, Length=1). Encodes the reason for the router restart, as one of the following: 0 (unknown), 1 (software restart), 2 (software reload/upgrade), or 3 (switch to redundant control processor). This TLV MUST always appear in a grace-LSA. (§2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c22 re-judge. Forbidden: a grace-LSA without the Graceful restart reason TLV (Type=2, Length=1). Positive: same origination unit asserts the literal 00 02 00 01 <reason> 00 00 00 after the period TLV for reason 2 and 0, body exactly 16 octets, so a wrong type number, length or omission goes red (recorded revert of v3 lsa_grace.go::tlvs). Negative under OWNER RULING 2: a period-only (600) v3 Grace-LSA through grInspectV6Update leaves the helper session and its grace end unchanged while the both-TLV control applies 600, isolating the hasReason check (author overlay; recorded revert of ::decodeGraceLSA). Tags moved off the OSPFv2 codec units (D-15).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5187HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_grace_tlv_test.go#L114) | unit/verify | revert, verified |
| positive | [`TestRFC5187OriginatedGraceLSACarriesPeriodAndReasonTLVs`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_grace_tlv_test.go#L33) | unit/verify | revert, verified |

### [`RFC5187-3.1-1`](#rfc5187-3.1-1)

Hence, to avoid network churn during graceful restart, the restarting router MUST preserve the LSA ID to prefix correspondence across graceful restarts. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, c11). Now proven through the real restart path (rfc5187_preserve_restart_test.go): engine A redistributes 2001:db8:5187::/48 (v6InjectExternal), prepareRestart persists into a store; engine B over the same store resumes (inRestart asserted). TestRFC5187LSAIDToPrefixPreservedAcrossRestart +: B maps the prefix to A's LSA ID without redistributing it (only restorePrefixLSIDs can put it there; a no-op capture or restore leaves no entry -> red). - same unit: a prefix first redistributed on B does not take the preserved ID and the preserved prefix keeps its own; an overlay mutants (go test -overlay, tree untouched, scratch/ospf-judge-mut) removing the redistV6Next bump in restorePrefixLSIDs turns it red ('took the preserved LSA ID 1'). Revert records on restorePrefixLSIDs observed red. Old gr_nvs_test.go tags (store round-trip only) remain, unproven.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestStaleRestartFactIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_gr_nvs_test.go#L110) | unit/verify | unproven |
| negative | [`TestRFC5187LSAIDToPrefixPreservedAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_preserve_restart_test.go#L98) | unit/verify | revert, verified |
| positive | [`TestRestartFactPersistsAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_gr_nvs_test.go#L60) | unit/verify | unproven |
| positive | [`TestRFC5187LSAIDToPrefixPreservedAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_preserve_restart_test.go#L85) | unit/verify | revert, verified |

### [`RFC5187-3.2-1`](#rfc5187-3.2-1)

Therefore, the OSPFv3 Interface ID, as described in section 3.1.2 of [OSPFv3], MUST be preserved by the restarting router across restarts. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (independent judge, c12). Section 3.2: 'the OSPFv3 Interface ID ... MUST be preserved by the restarting router across restarts.' The c11 vacuous negative is replaced: TestRFC5187InterfaceIDPreservedAcrossRestart runs on lo, whose kernel ifindex is the Interface ID a fresh start takes (grInterfaceID falls back to interfaceIndex, gr_preserve.go). - neither grInterfaceID(lo) nor the announced topology ID is the kernel ifindex; + the resumed engine announces the preserved 1041. Author overlay making restoreInterfaceIDs' maps.Copy a no-op reds the negative line ('lo resolves to its fresh-start Interface ID 1'), judge read the diff and the log. Records +/- revert on restoreInterfaceIDs observed. The unit needs a host lo interface (Linux naming) and Fatalf's without it. gr_nvs_test.go tags are supplementary (NVS round-trip).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestStaleRestartFactIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_gr_nvs_test.go#L114) | unit/verify | unproven |
| negative | [`TestRFC5187InterfaceIDPreservedAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_preserve_restart_test.go#L144) | unit/verify | revert, verified |
| positive | [`TestRestartFactPersistsAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_gr_nvs_test.go#L63) | unit/verify | unproven |
| positive | [`TestRFC5187InterfaceIDPreservedAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc5187_preserve_restart_test.go#L154) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5187.txt |
| Source fingerprint | 586d23e887e28896 |
| Record | rfc/extraction/rfc5187.json |
| Mapped sentences | 4 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 5187 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 5187, so its obligations are stated where they were written.
