# DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY - Software Version Capability for BGP

BGP Software Version capability code 75. Every requirement listed for DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 11 | of 11 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 11 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 11 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 11 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 11 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 21 | 21 of 22 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 11 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 11 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 11 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 11 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 11 | of 15 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 11 mandatory requirements: they add up to 11. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP Software Version capability code 75 |
| Automatic checks | Included |
| Requirements | 15 |
| Mandatory requirements (MUST level) | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tests linked to requirements | 22 |
| Recorded audit verdicts | 11 |
| Fault test records | 21 |
| Summary | `rfc/short/draft-abraitis-bgp-version-capability.md` |
| Requirement table | `rfc/requirements/draft-abraitis-bgp-version-capability.md` |
| RFC text | `rfc/drafts/draft-abraitis-bgp-version-capability.txt` |

## Automatic checks

Included in automatic checks: Software Version capability for BGP (code 75): eleven MUST-level requirements. Send side is implemented and gated by config -- encodeValue writes the constant ZeVersion as the whole Capability Value under the default encoding draft (encoding legacy prefixes a length octet, an owner-approved deviation, see Notes), and extractSoftverCapabilities declares code 75 only for a peer or group whose config carries a software-version key that is not disable or refuse (internal/component/bgp/plugins/softver/softver.go), which is 3-1, 3-2, 3-3, 3-6, 3-8 and 4-2. Receive side: parseCapability (internal/core/bgp/capability/capability.go) has no case for code 75, so a received capability becomes an Unknown nothing reads, which is how 3-4, 3-5, 3-7 and 4-1 are met. 3.1-1 requires RFC 9072, which is enrolled separately and Partial. No requirement carries a tagged test yet; the coverage rollup names them.

## Published support status

**Area:** BGP Software Version capability code 75

**Test verification:** 11 of 11 MUST proven

**Evidence:** unit 11

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 11 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **11** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (11):** [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1`](#draft-abraitis-bgp-version-capability-3-1), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2`](#draft-abraitis-bgp-version-capability-3-2), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3`](#draft-abraitis-bgp-version-capability-3-3), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4`](#draft-abraitis-bgp-version-capability-3-4), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5`](#draft-abraitis-bgp-version-capability-3-5), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6`](#draft-abraitis-bgp-version-capability-3-6), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7`](#draft-abraitis-bgp-version-capability-3-7), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8`](#draft-abraitis-bgp-version-capability-3-8), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1`](#draft-abraitis-bgp-version-capability-3.1-1), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1`](#draft-abraitis-bgp-version-capability-4-1), [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2`](#draft-abraitis-bgp-version-capability-4-2)

## Figures

- **MUST bar**: Ze verified 11
- **Overview bar**: Ze MUST verified 11, Not assessed 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze verified | 11 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 4 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/draft-abraitis-bgp-version-capability.json](../../../data/rfc/draft-abraitis-bgp-version-capability.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1` | If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use (§3) | MUST | 3 - Software Version Capability | **positive:** `unit/verify` [`TestSoftverConfigOptionEnables`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L27). **negative:** `unit/verify` [`TestSoftverConfigOptionDisables`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L37) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2` | a configuration option to enable or disable its use, and MUST default to disabled. (§3) | MUST | 3 - Software Version Capability | **positive:** `unit/verify` [`TestSoftverDefaultsToDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L46). **negative:** `unit/verify` [`TestSoftverDefaultIsNotABlanketRefusal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L55) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3` | The Capability Length for the Software Version Capability MUST be greater than zero. (§3) | MUST | 3 - Software Version Capability | **positive:** `unit/verify` [`TestSoftverCapabilityLengthIsGreaterThanZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L65). **negative:** `unit/verify` [`TestSoftverZeroCapabilityLengthIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L77) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4` | A value of zero SHALL be treated as an encoding error (§3) | SHALL | 3 - Software Version Capability | **positive:** `unit/verify` [`TestSoftverZeroValueIsAnEncodingError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L87). **negative:** `unit/verify` [`TestSoftverWellFormedValueIsNotAnEncodingError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L96) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5` | A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored. (§3) | MUST | 3 - Software Version Capability | **positive:** `unit/verify` [`TestSoftverZeroValueCapabilityIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L106). **negative:** `unit/verify` [`TestSoftverWellFormedCapabilityIsNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L125) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6` | The Version field MUST be encoded using UTF-8. (§3) | MUST | 3 - Software Version Capability | **positive:** `unit/verify` [`TestSoftverVersionFieldIsUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L140). **negative:** `unit/verify` [`TestSoftverNonUTF8VersionFieldIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L151) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7` | A receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences. (§3) | MUST NOT | 3 - Software Version Capability | **positive:** `unit/verify` [`TestSoftverInvalidUTF8IsNotInterpreted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L162). **negative:** `unit/verify` [`TestSoftverValidMultiByteUTF8IsInterpreted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L178) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8` | a sender MUST NOT generate advertising or other nonessential information within the product identifier. (§3) | MUST NOT | 3 - Software Version Capability | **positive:** `unit/verify` [`TestSoftverProductIdentifierCarriesNothingNonessential`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L211). **negative:** `unit/verify` [`TestSoftverNonessentialProductIdentifiersAreRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L224) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-9` | The Capability Length SHOULD be no greater than 64. (§3) | SHOULD | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-10` | A sender SHOULD limit generated product identifiers to what is necessary to identify the product; (§3) | SHOULD | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-11` | A sender SHOULD NOT generate information in product- version that is not a version identifier (§3) | SHOULD NOT | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-12` | It is NOT RECOMMENDED for use outside a single Autonomous System, or a set of Autonomous Systems under a common administration. (§3) | NOT RECOMMENDED | 3 - Software Version Capability | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1` | Implementations of this specification are REQUIRED Extended Optional Parameters Length for BGP OPEN Message support as defined in [RFC9072]. (§3.1) | REQUIRED | 3.1 - Capabilities Length Overflow | **positive:** `unit/verify` [`TestSoftwareVersionCapabilityUsesExtendedOptionalParameters`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/draft_abraitis_bgp_version_capability_softver_test.go#L44). **negative:** `unit/verify` [`TestSoftwareVersionCapabilityKeepsClassicFormUnderTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/draft_abraitis_bgp_version_capability_softver_test.go#L66) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1` | The Software Version Capability MUST only be used for displaying the version of a BGP speaker's router daemon to make troubleshooting easier. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestSoftwareVersionCapabilityIsRecordedForDisplay`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/draft_abraitis_bgp_version_capability_softver_test.go#L84). **negative:** `unit/verify` [`TestSoftwareVersionCapabilityDecidesNothing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/draft_abraitis_bgp_version_capability_softver_test.go#L36) |
| `DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2` | Enabling (i.e., turning on) this capability requires bouncing all existing BGP sessions and the feature MUST be explicitly configured before an implementation advertizes the Software Version Capability. (§4) | MUST | 4 - Operation | **positive:** `unit/verify` [`TestSoftverAdvertisedOnlyAfterExplicitConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L241). **negative:** `unit/verify` [`TestSoftverNotAdvertisedWithoutExplicitConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L252) |

## Gaps and untested MUSTs

DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1`](#draft-abraitis-bgp-version-capability-3-1)

If an implementation supports the inclusion of the capability, the implementation MUST include a configuration option to enable or disable its use (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a software-version capability no configuration option can switch on or off. TestSoftverConfigOptionEnables asserts extractSoftverCapabilities declares code 75 for mode enable, red if the option were ignored. Negative: TestSoftverConfigOptionDisables, mode disable declares nothing.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftverConfigOptionDisables`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestSoftverConfigOptionEnables`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L27) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-2`](#draft-abraitis-bgp-version-capability-3-2)

a configuration option to enable or disable its use, and MUST default to disabled. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: advertising code 75 by default. TestSoftverDefaultsToDisabled asserts a peer whose capability block has no software-version key gets no declaration, red if the capability were on by default. Negative: TestSoftverDefaultIsNotABlanketRefusal, the same peer with the key present gets code 75, so the empty answer is the default and not a refusal of everything.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftverDefaultIsNotABlanketRefusal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L55) | unit/verify | revert, verified |
| positive | [`TestSoftverDefaultsToDisabled`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L46) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-3`](#draft-abraitis-bgp-version-capability-3-3)

The Capability Length for the Software Version Capability MUST be greater than zero. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: sending code 75 with Capability Length zero. TestSoftverCapabilityLengthIsGreaterThanZero asserts the Capability Value encodeValue produces for the default draft encoding is longer than zero (require.Greater) and is exactly the bytes of ZeVersion, so the Capability Length is len(ZeVersion); red on an empty value. The legacy encoding is an owner-approved deviation (D-13) and is not what this verdict covers. Negative: TestSoftverZeroCapabilityLengthIsRefused, decodeSoftwareVersion(nil) returns ok false and no version.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftverZeroCapabilityLengthIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestSoftverCapabilityLengthIsGreaterThanZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L65) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-4`](#draft-abraitis-bgp-version-capability-3-4)

A value of zero SHALL be treated as an encoding error (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: reading a zero-length Capability Value as a valid version. TestSoftverZeroValueIsAnEncodingError asserts decodeSoftwareVersion([]byte{}) returns ok false, red if it decodes; the length check runs before the legacy-form test, so neither form reads an empty value. Negative: TestSoftverWellFormedValueIsNotAnEncodingError, the bare value zebgp decodes to zebgp, red if the error verdict were returned for every input.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftverWellFormedValueIsNotAnEncodingError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L96) | unit/verify | revert, verified |
| positive | [`TestSoftverZeroValueIsAnEncodingError`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L87) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-5`](#draft-abraitis-bgp-version-capability-3-5)

A value of zero SHALL be treated as an encoding error and the Capability MUST be ignored. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: acting on or displaying a zero-length capability. TestSoftverZeroValueCapabilityIsIgnored asserts RunCLIDecode of an empty value exits 1 with empty stdout, red if decodeSoftwareVersion accepted it and a version were shown. Its RunDecodeMode arm feeds the empty hex field invokePluginDecode writes for a zero-length value and asserts decoded unknown with no version; that arm is refused by the request parser before the decoder, so the CLI arm carries the discrimination. Negative: TestSoftverWellFormedCapabilityIsNotIgnored, the bare value zebgp is decoded and shown on both surfaces.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftverWellFormedCapabilityIsNotIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L125) | unit/verify | revert, verified |
| positive | [`TestSoftverZeroValueCapabilityIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L106) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-6`](#draft-abraitis-bgp-version-capability-3-6)

The Version field MUST be encoded using UTF-8. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a Version field that is not UTF-8. TestSoftverVersionFieldIsUTF8 asserts utf8.Valid over the whole Capability Value encodeValue writes for the default draft encoding, which is the version string with no length octet, and equality with ZeVersion; red on invalid UTF-8 or on any octet beside the string. The legacy encoding's length octet is the D-13 deviation and is outside this verdict. Negative: TestSoftverNonUTF8VersionFieldIsRefused, C3 28 61 is refused by decodeSoftwareVersion.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftverNonUTF8VersionFieldIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L151) | unit/verify | revert, verified |
| positive | [`TestSoftverVersionFieldIsUTF8`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L140) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-7`](#draft-abraitis-bgp-version-capability-3-7)

A receiving BGP speaker MUST NOT interpret invalid UTF-8 sequences. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: interpreting invalid UTF-8 received in the Version field. TestSoftverInvalidUTF8IsNotInterpreted asserts decodeSoftwareVersion returns ok false and an empty string for the value FF FE FD FC, and RunCLIDecode exits 1 with empty stdout, red if the bytes are rendered. decodeSoftwareVersion checks utf8.Valid on the legacy remainder before taking that form, and on the whole value otherwise, so neither form hands invalid bytes on. Negative: TestSoftverValidMultiByteUTF8IsInterpreted, the bare value Zé/0.1.0 decodes intact.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftverValidMultiByteUTF8IsInterpreted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L178) | unit/verify | revert, verified |
| positive | [`TestSoftverInvalidUTF8IsNotInterpreted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L162) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-8`](#draft-abraitis-bgp-version-capability-3-8)

a sender MUST NOT generate advertising or other nonessential information within the product identifier. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: advertising or other nonessential information in the product identifier ze sends. TestSoftverProductIdentifierCarriesNothingNonessential asserts the whole Capability Value encodeValue writes for the default draft encoding matches the product[/product-version] grammar with no spaces or prose (productIdentifierIsEssential), red if ZeVersion carried any. Negative: six identifiers carrying such information are refused by the same check.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftverNonessentialProductIdentifiersAreRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L224) | unit/verify | not verified |
| positive | [`TestSoftverProductIdentifierCarriesNothingNonessential`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L211) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1`](#draft-abraitis-bgp-version-capability-3.1-1)

Implementations of this specification are REQUIRED Extended Optional Parameters Length for BGP OPEN Message support as defined in [RFC9072]. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: no RFC 9072 Extended Optional Parameters Length support. TestSoftwareVersionCapabilityUsesExtendedOptionalParameters packs an OPEN whose capabilities exceed 255 octets with code 75 inside and asserts the FF FF markers, the two-octet length, and UnpackOpen reporting ExtendedParams with identical parameters, red if either direction lacks the extended form. Negative: under 255 octets the classic form is used.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftwareVersionCapabilityKeepsClassicFormUnderTheCeiling`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/draft_abraitis_bgp_version_capability_softver_test.go#L66) | unit/verify | revert, verified |
| positive | [`TestSoftwareVersionCapabilityUsesExtendedOptionalParameters`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/draft_abraitis_bgp_version_capability_softver_test.go#L44) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1`](#draft-abraitis-bgp-version-capability-4-1)

The Software Version Capability MUST only be used for displaying the version of a BGP speaker's router daemon to make troubleshooting easier. (§4)

Audit verdict: enforced (the tests do what the requirement demands), stale-unit: internal/core/bgp/capability/negotiated.go::Negotiate moved. Independent rejudge after 4594143b56. Draft Section 4: "The Software Version Capability MUST only be used for displaying the version of a BGP speaker's router daemon to make troubleshooting easier." Producers at HEAD conform: parseCapability returns code 75 as an Unknown holding a copy of the raw value; Negotiate's remote loop only sets peerCodes[75] for it (no type case matches Unknown), remoteOrder takes Multiprotocol entries alone; the reactor's RefusedCapabilities and RequiredCapabilities are filled only by applyCapMode (config_capabilities.go), which is never called with code 75, so validateCapabilityModes cannot act on it; the softver plugin's own refuse mode only suppresses advertisement and its decode path is display. Positive: exact Unknown code and bytes after parse, and PeerAdvertised(75) after Negotiate, which is what a display reads; a revert record (Negotiate body panics) goes red. Negative isolates code 75 (two OPENs identical but for one Unknown placed between the two Multiprotocol entries) and asserts Families with order, Identity, Encoding, Session and the named session fields, then the previously unchecked peerCodes: with.peerCodes must equal a copy of without.peerCodes plus {75: true} exactly, then the whole Negotiated (unexported fields included, via reflect equality) with peerCodes cleared. That closes the 10e10198ce finding: a Negotiate that marks any extra code (for example 70, which selects RFC 7313 handling via PeerAdvertised, or a refused code read by CheckRefusedCodes) when it sees 75 now goes red; the recorded mutant (also mark code 70 on 75) was observed red at that assertion. The family-order mutant record was replaced (one record per requirement, polarity and unit), but the in-order Families assertion with 75 between the Multiprotocol entries remains in the body, so that break is still caught. The positive and negative are distinct assertions, not one test with two tags. Residual, not a finding: proof is at the capability boundary; reactor-level non-use rests on the reads above (every session decision reads Negotiated or config lists that never carry 75).

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftwareVersionCapabilityDecidesNothing`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/draft_abraitis_bgp_version_capability_softver_test.go#L36) | unit/verify | mutant, verified |
| positive | [`TestSoftwareVersionCapabilityIsRecordedForDisplay`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/capability/draft_abraitis_bgp_version_capability_softver_test.go#L84) | unit/verify | revert, verified |

### [`DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2`](#draft-abraitis-bgp-version-capability-4-2)

Enabling (i.e., turning on) this capability requires bouncing all existing BGP sessions and the feature MUST be explicitly configured before an implementation advertizes the Software Version Capability. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: advertising code 75 without explicit configuration. TestSoftverNotAdvertisedWithoutExplicitConfiguration asserts no declaration for a peer with no capability block, an empty capability block, and a group without the key (assert.Empty), red if any advertises. Positive: an explicit mode enable declares code 75 with encodeValue's payload.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestSoftverNotAdvertisedWithoutExplicitConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L252) | unit/verify | revert, verified |
| positive | [`TestSoftverAdvertisedOnlyAfterExplicitConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/softver/draft_abraitis_bgp_version_capability_test.go#L241) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 draft walk, draft-abraitis-bgp-version-capability |
| Signed off | 2026-09-01 |
| Register | prose |
| Source | rfc/drafts/draft-abraitis-bgp-version-capability.txt |
| Source fingerprint | 472ca7a757b75dda |
| Record | rfc/extraction/draft-abraitis-bgp-version-capability.json |
| Mapped sentences | 9 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | skipped (front-matter) | Title block, Abstract, Status of This Memo and Copyright Notice. The Abstract carries the sentence that makes the whole mechanism optional and one boilerplate site; both are excluded below. |
| `1` | Introduction | 1 | reviewed | Introduction. Says data-center routers run different routing-software versions, that knowing them helps root-cause a fault, and that LLDP and CDP are awkward in containerized environments. Its one site restates the Abstract's optionality and is excluded below. No obligation. |
| `2` | Specification of Requirements | 0 | reviewed | Specification of Requirements. The BCP 14 key-words paragraph. It tells a reader how to read the rest and binds nobody, which is why the derivation raises no site here. |
| `3` | Software Version Capability | 7 | reviewed | Software Version Capability. The document's main normative section: seven sites, five mapped to DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3-1, 3-3, 3-4, 3-6, 3-7 and 3-8, and two excluded. It also assigns Capability Code 75 and defines the Capability Length and Capability Value fields, which are value definitions carried by the Wire Format section of rfc/short/draft-abraitis-bgp-version-capability.md. Six ids are declared unsourced here. Two are the second obligation of a sentence already mapped: 3-2, the MUST to default to disabled, shares site 3:1 with 3-1, and 3-5, the MUST to ignore a zero-length capability, shares site 3:4 with 3-4. The other four are advisory levels the capitalised MUST-level scan does not raise: 3-9 (the SHOULD that Capability Length be no greater than 64), 3-10 (the SHOULD to limit the product identifier, the first clause of site 3:7), 3-11 (the SHOULD NOT to put non-version information in product-version) and 3-12 (the NOT RECOMMENDED against use outside one Autonomous System). The opening sentence, that the document is not Standards Track but uses BCP 14 terminology, and the OPTIONAL sentence 'The inclusion of the Software Version Capability is OPTIONAL' state no obligation on a speaker. |
| `3.1` | Capabilities Length Overflow | 2 | reviewed | Capabilities Length Overflow. Two sites: 3.1:1 is mapped to DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-3.1-1 and 3.1:2 is excluded below. The rest recounts that RFC 5492 caps the BGP Capabilities Optional Parameter at 255 bytes and that exceeding it leaves no room for more important capabilities. |
| `4` | Operation | 2 | reviewed | Operation. Two sites, both mapped, to DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-1 and DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY-4-2. The remaining paragraph is the worked example of an operator correlating a bug with a routing-daemon version across upstream nodes. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records that IANA has assigned capability number 75 in the BGP Capability Codes registry, with the one-row table naming it. Binds IANA, not a speaker. |
| `6` | Security Considerations | 0 | reviewed | Security Considerations. States that the version should be treated as sensitive, that an attacker who learns it has an easier time, that a forged or snooped value is possible without an integrity- or confidentiality-providing transport, and that GTSM or a firewall on TCP 179 limits the exposure. Every sentence uses a lowercase 'should' the document's own Section 2 gives no normative level, and the derivation raises no site. No obligation on a speaker. |
| `A` | Appendix A | 0 | skipped (appendix-non-normative) | Appendix A. Implementation Report. The RFC 7942 running-code list -- FRRouting, GoBGP, freeRouter and ExaBGP commits -- plus three figures of sample CLI output. No obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative, and permissive rather than obligatory: it states that an implementation need not advertise the version and need not process a received one. It directs no behavior, and it is the sentence that makes both halves of the mechanism optional. Ze offers the send half and declines the receive half, which is why the three receiver requirements are met by ignoring rather than by parsing. | Implementations are not required to advertise the version nor to process received advertisements. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust boilerplate the splitter did not strip. The sentence binds a person who extracts Code Components from the document into other software and tells them to carry the Revised BSD License text. It directs no BGP speaker and describes no protocol behavior. Its 'must' is lowercase, so it is raised only by the case-insensitive modal scan the 'prose' register uses. | Code Components extracted from this document must include Revised BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Revised BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction's restatement of the Abstract sentence at site front:1, in the same permissive shape: implementations 'are not required to' advertise or to process. Indicative, and it directs no behavior. It is not recorded as a duplicate-of, because duplicate-of names an id another site maps and no site maps this sentence: it states no obligation for an id to hold. | Implementations are not required to advertise their software version nor to process it on receipt. |
| `3:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 5492, which the sentence cites: a speaker that receives a capability it does not recognize or support must ignore it, RFC 5492 Section 3. The sentence is a back-reference telling a reader what already governs an unrecognized code 75, and its 'must' is lowercase. RFC 5492 is enrolled and that obligation is gated there as RFC5492-3-2. | An implementation that does not recognize or support the Software Version Capability but receives one must ignore it, as described in Section 3 of [RFC5492]. |
| `3.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative, and a security observation rather than a directive: it states that a rogue node can deny the advertisement of other capabilities by not excluding this one, and that the risk is not new to BGP. The words 'as required in Section 3.1' are a back-reference to the sentence at site 3.1:1, which this walk maps; they impose nothing of their own. | A rogue node can prevent the proper operation of a BGP session, or the advertisement of other Capabilities, by not excluding the Software Version Capability as required in Section 3.1. |

## Superseded

No document obsoletes DRAFT-ABRAITIS-BGP-VERSION-CAPABILITY, so its obligations are stated where they were written.
