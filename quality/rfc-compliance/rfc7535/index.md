# RFC 7535 - AS112 Redirection Using DNAME

EMPTY.AS112.ARPA. Every requirement listed for RFC 7535, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

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
| Mandatory requirements (MUST level) | 1 | of 7 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

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
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | EMPTY.AS112.ARPA |
| Automatic checks | Included |
| Requirements | 7 |
| Mandatory requirements (MUST level) | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc7535.md` |
| Requirement table | `rfc/requirements/rfc7535.md` |
| RFC text | `rfc/full/rfc7535.txt` |

## Automatic checks

Included in automatic checks: AS112 Redirection Using DNAME (EMPTY.AS112.ARPA): its sole MUST, RFC7535-6-1, states DNAME support on the AS112 node is never required -- the DNAME records live in the redirecting zones, not on the node, which only answers authoritatively for EMPTY.AS112.ARPA. ze fills the AS112 node role (internal/plugins/as112): it serves the empty zone, announces the DNAME-redirection anycast prefixes and implements no DNAME record processing, which meets the row, but no test carries it, so it is a gap in proof. The 3.1/3.2/4 SHOULDs and MAYs (address/zone configuration, DNSSEC signing) are operator/zone-admin guidance, not gated.

## Published support status

**Area:** EMPTY.AS112.ARPA

**Proof:** 0 of 1 MUST proven

**Evidence:** -

**Unmet:** RFC7535-6-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 1 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **1** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (1):** [`RFC7535-6-1`](#rfc7535-6-1)

## Figures

- **MUST bar**: Not built 1
- **Overview bar**: Not assessed 6, Not built 1

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
| Not assessed | 0 |
| Not built | 1 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 6 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7535.json](../../../data/rfc/rfc7535.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7535-6-1` | DNAME support on AS112 nodes themselves is never required under this proposal. (§6) | MUST | 6 - DNAME Deployment Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/plugins/as112/server.go::answerQuery meets it, since ze's AS112 node answers authoritatively for EMPTY.AS112.ARPA and processes no DNAME record, the DNAME records living in the redirecting zones; no test asserts it (internal/plugins/as112/integration_linux_test.go resolves foo.empty.as112.arpa under no tag) |
| `RFC7535-3.1-1` | Guidance to operators of AS112 nodes is extended to include configuration of the 192.31.196.1 and 2001:4:112::1 addresses, and the corresponding announcement of covering routes for those addresses (§3.1) | SHOULD | 3.1 - Extensions to Support DNAME Redirection | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-3.1-2` | Guidance to operators of AS112 nodes is extended to include configuration of the 192.31.196.1 and 2001:4:112::1 addresses, and the corresponding announcement of covering routes for those addresses, and to host the EMPTY.AS112.ARPA zone. (§3.1) | SHOULD | 3.1 - Extensions to Support DNAME Redirection | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-3.1-3` | IPv4-only AS112 nodes should only configure the 192.31.196.1 nameserver address (§3.1) | SHOULD | 3.1 - Extensions to Support DNAME Redirection | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-3.1-4` | IPv6-only AS112 nodes should only configure the 2001:4:112::1 nameserver address. (§3.1) | SHOULD | 3.1 - Extensions to Support DNAME Redirection | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-4-1` | Existing guidance to AS112 server operators to accept and respond to queries directed at the PRISONER.IANA.ORG, BLACKHOLE-1.IANA.ORG, and BLACKHOLE-2.IANA.ORG nameservers should continue to be followed, and no changes to the delegation of existing zones hosted on AS112 servers should occur. (§4) | SHOULD | 4 - Continuity of AS112 Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-3.2-1` | DNAME resource records deployed for this purpose can be signed with DNSSEC [RFC4033], providing a secure means of authenticating the legitimacy of each redirection. (§3.2) | MAY | 3.2 - Redirection of Query Traffic to AS112 Servers | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7535-6-1`](#rfc7535-6-1) DNAME support on AS112 nodes themselves is never required under this proposal. (§6) | {gap}, no test | gap in proof: internal/plugins/as112/server.go::answerQuery meets it, since ze's AS112 node answers authoritatively for EMPTY.AS112.ARPA and processes no DNAME record, the DNAME records living in the redirecting zones; no test asserts it (internal/plugins/as112/integration_linux_test.go resolves foo.empty.as112.arpa under no tag) |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC7535-6-1`](#rfc7535-6-1)

DNAME support on AS112 nodes themselves is never required under this proposal. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7535-6-1, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc7535 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc7535.txt |
| Source fingerprint | c4c8927061eec869 |
| Record | rfc/extraction/rfc7535.json |
| Mapped sentences | 1 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | reviewed | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. Walked rather than skipped because the site scan attributes one site here, the IETF Trust Legal Provisions boilerplate, excluded below. The Abstract states that the document describes a way to redirect queries to AS112 servers using DNAME. Nothing before section 1 binds a nameserver or its operator. |
| `1` | Introduction | 1 | reviewed | Introduction. The first three paragraphs are RFC 7534's introduction repeated word for word. The rest states the problem this document solves: AS112 operators are only loosely coordinated, so adding or removing a directly delegated zone is hard to do accurately and hard to test remotely across an anycast cloud, and a zone delegated to nameservers not yet configured for it is a lame delegation. Its one site, 1:1, is the existing Direct Delegation obligation quoted from RFC 7534 and is excluded cross-document below. The closing paragraphs state what this document defines: DNAME redirection deployable alongside unmodified AS112 nodes, usable by any zone administrator without coordinating with AS112 operators. |
| `2` | Design Overview | 0 | reviewed | Design Overview. States the design as fact: EMPTY.AS112.ARPA is delegated to the single nameserver BLACKHOLE.AS112.ARPA at 192.31.196.1 and 2001:4:112::1, and each address was chosen so a single covering prefix (a /24 and a /48) carries it with no other address in use inside that prefix, which is what makes the anycast distribution work. Two lowercase-'should' sentences follow and both are declared elsewhere: 'Some or all of the existing AS112 nodes should be extended to support these new nameserver addresses and to host the EMPTY.AS112.ARPA zone' is section 3.1's guidance, declared unsourced there as RFC7535-3.1-1 and RFC7535-3.1-2; and 'Each part of the DNS namespace for which it is desirable to sink queries at AS112 nameservers should be redirected to the EMPTY.AS112.ARPA zone using DNAME' binds the redirecting zone's administrator, which the section itself says by pointing at 3.2 for that guidance. Ze's constants for the two addresses are anycastV4DNAMERedirectionAddr and anycastV6DNAMERedirectionAddr in internal/plugins/as112/register.go. |
| `3` | AS112 Operations, the parent heading | 0 | reviewed | AS112 Operations, the parent heading. No text of its own. |
| `3.1` | Extensions to Support DNAME Redirection | 0 | reviewed | Extensions to Support DNAME Redirection. The section that carries every id binding an AS112 node, and no site: 'Guidance to operators of AS112 nodes is extended to include configuration of the 192.31.196.1 and 2001:4:112::1 addresses, and the corresponding announcement of covering routes for those addresses, and to host the EMPTY.AS112.ARPA zone' is indicative and is declared here as RFC7535-3.1-1 and RFC7535-3.1-2. 'IPv4-only AS112 nodes should only configure the 192.31.196.1 nameserver address; IPv6-only AS112 nodes should only configure the 2001:4:112::1 nameserver address' is RFC7535-3.1-3 and RFC7535-3.1-4. The last paragraph states that one operator implementing the extension is enough for the mechanism to work, that more is better for capacity and distribution, and that not all operators need to change: an observation about the deployment, obliging nobody. Ze implements the whole of it: serverEndpoints (internal/plugins/as112/register.go) binds the two addresses, coveringPrefixesFor (internal/plugins/as112/redistribute.go) announces 192.31.196.0/24 and 2001:4:112::/48, buildServedZones (internal/plugins/as112/zones.go) holds empty.as112.arpa, and familiesFor together with the address-family config leaf gives the single-stack cases. |
| `3.2` | Redirection of Query Traffic to AS112 Servers | 1 | reviewed | Redirection of Query Traffic to AS112 Servers. Guidance to the administrator of a redirecting zone, not to an AS112 node: install a DNAME at the redirection point, with the worked example of 2.0 IN DNAME EMPTY.AS112.ARPA. in 192.IN-ADDR.ARPA for TEST-NET-1. States that there is no practical limit on the number of redirections and that any of them can be removed at any time by that administrator. Its one site, 3.2:1, is the sentence saying deployed AS112 nodes need no change to support further redirections, excluded below. The closing sentence, 'DNAME resource records deployed for this purpose can be signed with DNSSEC [RFC4033], providing a secure means of authenticating the legitimacy of each redirection', is declared here as RFC7535-3.2-1 at [MAY]; it binds the same zone administrator, and ze hosts no redirecting zone. |
| `4` | Continuity of AS112 Operations | 0 | reviewed | Continuity of AS112 Operations. One lowercase-'should' sentence, declared unsourced here as RFC7535-4-1: 'Existing guidance to AS112 server operators to accept and respond to queries directed at the PRISONER.IANA.ORG, BLACKHOLE-1.IANA.ORG, and BLACKHOLE-2.IANA.ORG nameservers should continue to be followed, and no changes to the delegation of existing zones hosted on AS112 servers should occur.' The second paragraph speculates that the RFC 1918 delegations might one day be replaced by DNAME redirection and the three nameservers retired, then says explicitly that this document gives the IANA no such direction. Ze keeps both services: buildServedZones (internal/plugins/as112/zones.go) holds the nineteen Direct Delegation reverse zones beside empty.as112.arpa, and hostAddresses (internal/plugins/as112/register.go) binds the Direct Delegation anycast addresses beside the DNAME-redirection ones. |
| `5` | Candidate Zones for AS112 Redirection | 1 | reviewed | Candidate Zones for AS112 Redirection. States that every zone listed in RFC 6303 is a candidate, and closes by saying the document 'is simply concerned with provision of the AS112 redirection service and does not specify that any particular AS112 redirection be put in place'. Its one site, 5:1, is the sentence explaining why any zone owner can use the mechanism, excluded below. Choosing a zone to redirect binds that zone's owner, and the candidate list belongs to RFC 6303. |
| `6` | DNAME Deployment Considerations | 2 | reviewed | DNAME Deployment Considerations. Reviews whether DNAME is deployed widely enough for the mechanism to work, with Appendix A as the measurement. States that DNAME support on the AUTHORITATIVE server is a prerequisite for DNAME redirection, that a resolver without DNAME accepts and caches the synthesised CNAME RFC 6672 supplies, and that negative caching of the CNAME target follows EMPTY.AS112.ARPA's own parameters (RFC 2308), so every redirected name is negatively cached alike. Two sites: 6:1 binds a DNSSEC-validating resolver and is excluded below; 6:2 is the section's last sentence and is mapped to RFC7535-6-1, the only MUST-level id this summary declares. |
| `7` | IAB Statement Regarding This .ARPA Request | 0 | reviewed | IAB Statement Regarding This .ARPA Request. Records that the IAB approves the delegation of AS112 in the ARPA domain and has asked IANA to provision AS112.ARPA under RFC 3172, while taking no architectural or technical position on the specification. It binds the IAB and the IANA, and states no protocol behavior. |
| `8` | IANA Considerations, the parent heading | 0 | skipped (iana) | IANA Considerations, the parent heading. |
| `8.1` | Address Assignment | 0 | skipped (iana) | Address Assignment. The IANA registrations of 192.31.196.0/24 as AS112-v4 and 2001:4:112::/48 as AS112-v6 in the IPv4 and IPv6 Special-Purpose Address Registries [RFC6890]. IANA actions. The two prefixes themselves are what an AS112 node announces, which is RFC7535-3.1-1 on section 3.1. |
| `8.2` | Hosting of AS112.ARPA | 0 | skipped (iana) | Hosting of AS112.ARPA. States that the IANA hosts and signs the AS112.ARPA zone with infrastructure of its choosing and may adjust the SOA RDATA, and gives the zone content: the IANA-SERVERS.NET NS set, the A and AAAA of BLACKHOLE, and the NS delegations of HOSTNAME and EMPTY to BLACKHOLE. It binds the IANA. An AS112 node operator hosts the delegated EMPTY.AS112.ARPA zone, never this parent. |
| `8.3` | Delegation of AS112.ARPA | 0 | skipped (iana) | Delegation of AS112.ARPA. Records that the IANA has arranged the delegation from the ARPA zone by normal procedure and that the whois contact is specified by the IAB under RFC 3172. An IANA action. |
| `9` | Security Considerations | 0 | reviewed | Security Considerations. Two sentences: this document presents no known additional security concerns to the Internet, and RFC 7534 holds the security considerations for AS112 service in general. No countermeasure is directed at a nameserver, and the considerations it points at are walked in rfc/extraction/rfc7534.json section 8. |
| `10` | References, the parent heading | 0 | skipped (references) | References, the parent heading. |
| `10.1` | not stated | 0 | skipped (references) | Normative References: RFC 1918, RFC 2308, RFC 3172, RFC 4033, RFC 4786, RFC 6303, RFC 6672, RFC 6890 and RFC 7534. |
| `10.2` | Informative References: RFC 1035, RFC 2860 and RFC 5737 | 0 | skipped (references) | Informative References: RFC 1035, RFC 2860 and RFC 5737. |
| `A` | Appendix A, Assessing Support for DNAME in the Real World | 0 | skipped (appendix-non-normative) | Appendix A, Assessing Support for DNAME in the Real World. The heading of the measurement that backs section 6's claim about DNAME deployment. It reports an experiment and directs nobody. |
| `A.1` | Methodology | 0 | skipped (appendix-non-normative) | Methodology. Describes the four URLs loaded into a browser by an advertisement-borne script, the zone contents behind them, the 10-second timer, and how the results were reported. A description of an experiment, not of AS112 service. |
| `A.2` | Results | 0 | skipped (appendix-non-normative) | Results. 338,478 recorded results from 2013-10-10 and 2013-10-11: at most 1.9% of tested clients failed to resolve a name behind a DNAME, against a 2.8% failure rate on the control URL, so the authors conclude there is no evidence of consistent DNAME resolution failure. A measurement, obliging nobody. The derivation attributes the trailing Acknowledgements and Authors' Addresses to this section, and neither binds anybody either. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the IETF Trust Legal Provisions paragraph of the Copyright Notice. Its lowercase 'must' binds a party extracting Code Components from the document text, and directs no DNS or BGP behavior. RFC 7535's figures are zone-file fragments and an experiment's URL list, not code components ze reuses. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation belongs to RFC 7534, which the paragraph directly above this sentence cites: 'The AS112 project is described in detail in [RFC7534].' The sentence repeats RFC 7534 section 3.5's requirement so that the next two paragraphs can state the problem this document solves -- a zone delegated to nameservers that were not configured for it in advance is a lame delegation, and coordinating that configuration across loosely coupled AS112 operators is hard. rfc/short/rfc7534.md declares it as RFC7534-3.5-1 and its own text names this restatement: '(section 3.5, also restated in RFC 7535 section 1)'. rfc/extraction/rfc7534.json declares it unsourced on section 3.5, so the obligation is bound there and is not left to this document. | The AS112 nameservers (PRISONER.IANA.ORG, BLACKHOLE-1.IANA.ORG, and BLACKHOLE-2.IANA.ORG) are required to answer authoritatively for each and every zone that is delegated to them. |
| `3.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The keyword is in a NEGATION: the sentence says nothing is required. 'No changes to deployed AS112 nodes incorporating the extensions described in this document are required to support additional redirections' states a property of the mechanism -- the DNAME lives in the redirecting zone, so adding or dropping a redirection touches that zone and never the sink -- and it directs no node to do anything. It is not a duplicate of RFC7535-6-1 either: that id says DNAME PROCESSING is never required on the node, while this says no RECONFIGURATION is required per redirection. Two different absences, neither of them an obligation. | No changes to deployed AS112 nodes incorporating the extensions described in this document are required to support additional redirections. |
| `5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The keyword is in a NEGATION again, and it is the subordinate clause of a sentence whose main clause states reach rather than obligation: 'Since no pre-provisioning is required on the part of AS112 operators ..., this mechanism supports AS112 redirection by any zone owner in the DNS.' It explains why any zone owner can use the mechanism. The section's own closing sentence confirms that nobody is directed here: the document 'does not specify that any particular AS112 redirection be put in place'. | Since no pre-provisioning is required on the part of AS112 operators to facilitate sinking of any name in the DNS namespace by AS112 infrastructure, this mechanism supports AS112 redirection by any zone owner in the DNS. |
| `6:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DNSSEC-validating recursive resolver, a role ze does not implement. The sentence says such a resolver is required to implement DNAME and should therefore not use a synthesised CNAME RR, which is an obligation RFC 6672 and RFC 4035 place on the validator, quoted here to show that signing the redirection point stays useful. Ze's AS112 node is authoritative-only with recursion disabled (RFC7534-3.5-4) and answers for EMPTY.AS112.ARPA rather than for a redirecting zone, so it neither validates nor emits a DNAME. Ze's own resolver is a STUB: dnssecDecision (internal/component/resolve/dns/resolver.go) sets the EDNS0 DO bit and reads an upstream SERVFAIL as the failure signal, relying on a validating upstream rather than validating a chain itself. | Validating resolvers (i.e., those requesting and processing DNSSEC [RFC4033] metadata) are required to implement DNAME and hence should not make use of synthesised CNAME RRs. |

## Superseded

No document obsoletes RFC 7535, so its obligations are stated where they were written.
