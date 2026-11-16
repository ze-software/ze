# RFC 6549 - OSPFv2 Multi-Instance Extensions

No entry in the public support list. Every requirement listed for RFC 6549, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 1 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 1 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 1 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 1 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 1 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 0 | 0 of 2 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |
| Audit verdicts | 1 | of 1 mandatory requirements reviewed | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 1 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 1 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 1 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 1 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 1 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 1 | of 4 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 1 mandatory requirements: they add up to 1. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | No entry in the public support list |
| Automatic checks | Included |
| Requirements | 4 |
| Mandatory requirements (MUST level) | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tests linked to requirements | 2 |
| Recorded audit verdicts | 1 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc6549.md` |
| Requirement table | `rfc/requirements/rfc6549.md` |
| RFC text | `rfc/full/rfc6549.txt` |

## Automatic checks

Included in automatic checks: OSPFv2 Multi-Instance Extensions: the sole MUST (RFC6549-2-1) requires a received OSPFv2 packet whose Instance ID does not match one configured on the receiving interface to be discarded before any handler runs. The per-instance demux lives in the shared dispatcher (internal/plugins/ospf/dispatcher.go, `h.InstanceID != instanceID`), one engine per Instance ID (internal/plugins/ospf/multi_instance.go). The positive/negative pair drives dispatch() directly: mismatched Instance IDs {0,1,4,6,255} are dropped before the handler, while the engine's own Instance ID is delivered, so the demux is not a blanket drop. The 5-1/6-1 SHOULDs and 3-1 MAY are backward-compat guidance and config surface, not gated.

## Published support status

No entry in the public support list, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6549.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **1** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC6549-2-1`](#rfc6549-2-1)

## Figures

- **MUST bar**: Tests awaiting verification 1
- **Overview bar**: Tests awaiting verification 1, Not assessed 3

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 1 |
| MUST rows kept (the MUST bar) | 1 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 3 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 3 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc6549.json](../../../data/rfc/rfc6549.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6549-2-1` | Received packets with an Instance ID not equal to one of the Instance IDs corresponding to one of the configured OSPFv2 Instances for the receiving interface MUST be discarded. (§2, §3.1) -- `internal/plugins/ospf/dispatcher.go`: `h.InstanceID != instanceID` discard, before any handler; one engine per Instance ID in `internal/plugins/ospf/multi_instance.go`; spec-ospf-ext-12 | MUST | 2 | **positive:** `unit/verify` [`TestDispatchDropsMismatchedInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6549_instance_test.go#L469). **negative:** `unit/verify` [`TestDispatchDropsMismatchedInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6549_instance_test.go#L482) |
| `RFC6549-6-1` | Consequently, it is recommended that implementations that implement this specification and the OSPF MIB also implement SNMP Notification filtering as specified in Section 6 of [RFC3413]. (§6) -- N/A: Ze has no OSPF SNMP MIB surface, so there is nothing to filter; recorded as a Known Limitation | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6549-5-1` | OSPFv2 routers not supporting this specification should only support the default instance (§5) -- Ze at Instance ID 0 is bit-for-bit compatible with base OSPFv2: `internal/plugins/ospf/packet/header.go`, `TestHeaderInstanceZeroUnchanged` | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6549-3-1` | The OSPFv2 Interface Instance ID has a default value of 0. Setting it to a non-zero value may be accomplished through configuration. (§3) -- the per-interface `instance-id` leaf-list in `internal/plugins/ospf/yang/ze-ospf-conf.yang`; spec-ospf-ext-12 | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6549 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC6549-2-1`](#rfc6549-2-1)

Received packets with an Instance ID not equal to one of the Instance IDs corresponding to one of the configured OSPFv2 Instances for the receiving interface MUST be discarded. (§2, §3.1) -- `internal/plugins/ospf/dispatcher.go`: `h.InstanceID != instanceID` discard, before any handler; one engine per Instance ID in `internal/plugins/ospf/multi_instance.go`; spec-ospf-ext-12

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) delivering a packet whose Instance ID differs from the engine's to a handler; (b) TestDispatchDropsMismatchedInstance: handled != 0 after IDs {0,1,4,6,255} to instance 5 fails; negative: the matching ID 5 reaches the handler (handled == 1). One engine per Instance ID covers several configured instances on one interface.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestDispatchDropsMismatchedInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6549_instance_test.go#L482) | unit/verify | not verified |
| positive | [`TestDispatchDropsMismatchedInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc6549_instance_test.go#L469) | unit/verify | not verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc6549.txt |
| Source fingerprint | d65193f7ba300cc9 |
| Record | rfc/extraction/rfc6549.json |
| Mapped sentences | 1 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 1 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.1 restates the discard rule the Instance ID field definition in Section 2 already states: 'Received packets with an Instance ID not equal to one of the configured OSPFv2 Instance IDs on the receiving interface MUST be discarded'. The summary declares one requirement for both occurrences and cites them together as (§2, §3.1). | Received packets with an Instance ID not equal to one of the configured OSPFv2 Instance IDs on the receiving interface MUST be discarded. |

## Superseded

No document obsoletes RFC 6549, so its obligations are stated where they were written.
