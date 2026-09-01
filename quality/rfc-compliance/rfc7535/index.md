# RFC 7535 - AS112 Redirection Using DNAME

Supported. Every requirement this repository extracted from RFC 7535, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 1 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 1 | of 1 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 1 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 1 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7535.md` |
| Requirement shard | `rfc/requirements/rfc7535.md` |
| RFC text | `rfc/full/rfc7535.txt` |

## Enrolment

Enrolled: AS112 Redirection Using DNAME (EMPTY.AS112.ARPA): its sole MUST, RFC7535-6-1, states DNAME support on the AS112 node is never required -- the DNAME records live in the redirecting zones, not on the node, which only answers authoritatively for EMPTY.AS112.ARPA. {not-applicable}: ze's AS112 node serves the empty zone (internal/plugins/as112) and announces the DNAME-redirection anycast prefixes but implements no DNAME record processing, so the "never required" statement imposes no gated obligation. The 3.1/3.2/4 SHOULDs and MAYs (address/zone configuration, DNSSEC signing) are operator/zone-admin guidance, not gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

EMPTY.AS112.ARPA DNAME-redirection zone.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **1** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (1):** [`RFC7535-6-1`](#rfc7535-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7535-6-1` | DNAME support on the AS112 node itself is never required under this proposal (§6) | MUST | 6 - DNAME Deployment Considerations | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** RFC 7535 places the DNAME records in the redirecting zones, not on the AS112 node -- the node only answers authoritatively for EMPTY.AS112.ARPA. ze's AS112 node does exactly that (internal/plugins/as112/server.go announces the DNAME-redirection anycast addresses and internal/plugins/as112/integration_linux_test.go resolves foo.empty.as112.arpa) and implements no DNAME record processing, so this "DNAME is never required on the node" statement imposes no gated obligation on ze |
| `RFC7535-3.1-1` | An AS112 node that implements this extension configures the 192.31.196.1 and 2001:4:112::1 nameserver addresses and announces covering BGP routes for them (§3.1) | SHOULD | 3.1 - Extensions to Support DNAME Redirection | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-3.1-2` | An AS112 node that implements this extension hosts the EMPTY.AS112.ARPA zone (§3.1) | SHOULD | 3.1 - Extensions to Support DNAME Redirection | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-3.1-3` | An IPv4-only AS112 node configures only the 192.31.196.1 address (§3.1) | SHOULD | 3.1 - Extensions to Support DNAME Redirection | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-3.1-4` | An IPv6-only AS112 node configures only the 2001:4:112::1 address (§3.1) | SHOULD | 3.1 - Extensions to Support DNAME Redirection | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-4-1` | Existing guidance to accept and answer queries at PRISONER.IANA.ORG, BLACKHOLE-1.IANA.ORG, and BLACKHOLE-2.IANA.ORG continues unchanged; no existing zone delegations are altered by this document (§4) | SHOULD | 4 - Continuity of AS112 Operations | **positive:** no positive test. **negative:** no negative test |
| `RFC7535-3.2-1` | DNAME resource records installed by a redirecting zone administrator may be signed with DNSSEC (§3.2) | MAY | 3.2 - Redirection of Query Traffic to AS112 Servers | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7535-6-1`](#rfc7535-6-1) DNAME support on the AS112 node itself is never required under this proposal (§6) | no test | no test carries this requirement id; annotated {not-applicable}: RFC 7535 places the DNAME records in the redirecting zones, not on the AS112 node -- the node only answers authoritatively for EMPTY.AS112.ARPA. ze's AS112 node does exactly that (internal/plugins/as112/server.go announces the DNAME-redirection anycast addresses and internal/plugins/as112/integration_linux_test.go resolves foo.empty.as112.arpa) and implements no DNAME record processing, so this "DNAME is never required on the node" statement imposes no gated obligation on ze |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7535-6-1`](#rfc7535-6-1)

DNAME support on the AS112 node itself is never required under this proposal (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7535-6-1, so no unit is bound to it.

## Extraction sign-off

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
| `front` | not stated | 1 | walked | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. Walked rather than skipped because the site scan attributes one site here, the IETF Trust Legal Provisions boilerplate, excluded below. The Abstract states that the document describes a way to redirect queries to AS112 servers using DNAME. Nothing before section 1 binds a nameserver or its operator. |
| `1` | Introduction | 1 | walked | Introduction. The first three paragraphs are RFC 7534's introduction repeated word for word. The rest states the problem this document solves: AS112 operators are only loosely coordinated, so adding or removing a directly delegated zone is hard to do accurately and hard to test remotely across an anycast cloud, and a zone delegated to nameservers not yet configured for it is a lame delegation. Its one site, 1:1, is the existing Direct Delegation obligation quoted from RFC 7534 and is excluded cross-document below. The closing paragraphs state what this document defines: DNAME redirection deployable alongside unmodified AS112 nodes, usable by any zone administrator without coordinating with AS112 operators. |
| `2` | Design Overview | 0 | walked | Design Overview. States the design as fact: EMPTY.AS112.ARPA is delegated to the single nameserver BLACKHOLE.AS112.ARPA at 192.31.196.1 and 2001:4:112::1, and each address was chosen so a single covering prefix (a /24 and a /48) carries it with no other address in use inside that prefix, which is what makes the anycast distribution work. Two lowercase-'should' sentences follow and both are declared elsewhere: 'Some or all of the existing AS112 nodes should be extended to support these new nameserver addresses and to host the EMPTY.AS112.ARPA zone' is section 3.1's guidance, declared unsourced there as RFC7535-3.1-1 and RFC7535-3.1-2; and 'Each part of the DNS namespace for which it is desirable to sink queries at AS112 nameservers should be redirected to the EMPTY.AS112.ARPA zone using DNAME' binds the redirecting zone's administrator, which the section itself says by pointing at 3.2 for that guidance. Ze's constants for the two addresses are anycastV4DNAMERedirectionAddr and anycastV6DNAMERedirectionAddr in internal/plugins/as112/register.go. |
| `3` | AS112 Operations, the parent heading | 0 | walked | AS112 Operations, the parent heading. No text of its own. |
| `3.1` | Extensions to Support DNAME Redirection | 0 | walked | Extensions to Support DNAME Redirection. The section that carries every id binding an AS112 node, and no site: 'Guidance to operators of AS112 nodes is extended to include configuration of the 192.31.196.1 and 2001:4:112::1 addresses, and the corresponding announcement of covering routes for those addresses, and to host the EMPTY.AS112.ARPA zone' is indicative and is declared here as RFC7535-3.1-1 and RFC7535-3.1-2. 'IPv4-only AS112 nodes should only configure the 192.31.196.1 nameserver address; IPv6-only AS112 nodes should only configure the 2001:4:112::1 nameserver address' is RFC7535-3.1-3 and RFC7535-3.1-4. The last paragraph states that one operator implementing the extension is enough for the mechanism to work, that more is better for capacity and distribution, and that not all operators need to change: an observation about the deployment, obliging nobody. Ze implements the whole of it: serverEndpoints (internal/plugins/as112/register.go) binds the two addresses, coveringPrefixesFor (internal/plugins/as112/redistribute.go) announces 192.31.196.0/24 and 2001:4:112::/48, buildServedZones (internal/plugins/as112/zones.go) holds empty.as112.arpa, and familiesFor together with the address-family config leaf gives the single-stack cases. |
| `3.2` | Redirection of Query Traffic to AS112 Servers | 1 | walked | Redirection of Query Traffic to AS112 Servers. Guidance to the administrator of a redirecting zone, not to an AS112 node: install a DNAME at the redirection point, with the worked example of 2.0 IN DNAME EMPTY.AS112.ARPA. in 192.IN-ADDR.ARPA for TEST-NET-1. States that there is no practical limit on the number of redirections and that any of them can be removed at any time by that administrator. Its one site, 3.2:1, is the sentence saying deployed AS112 nodes need no change to support further redirections, excluded below. The closing sentence, 'DNAME resource records deployed for this purpose can be signed with DNSSEC [RFC4033], providing a secure means of authenticating the legitimacy of each redirection', is declared here as RFC7535-3.2-1 at [MAY]; it binds the same zone administrator, and ze hosts no redirecting zone. |
| `4` | Continuity of AS112 Operations | 0 | walked | Continuity of AS112 Operations. One lowercase-'should' sentence, declared unsourced here as RFC7535-4-1: 'Existing guidance to AS112 server operators to accept and respond to queries directed at the PRISONER.IANA.ORG, BLACKHOLE-1.IANA.ORG, and BLACKHOLE-2.IANA.ORG nameservers should continue to be followed, and no changes to the delegation of existing zones hosted on AS112 servers should occur.' The second paragraph speculates that the RFC 1918 delegations might one day be replaced by DNAME redirection and the three nameservers retired, then says explicitly that this document gives the IANA no such direction. Ze keeps both services: buildServedZones (internal/plugins/as112/zones.go) holds the nineteen Direct Delegation reverse zones beside empty.as112.arpa, and hostAddresses (internal/plugins/as112/register.go) binds the Direct Delegation anycast addresses beside the DNAME-redirection ones. |
| `5` | Candidate Zones for AS112 Redirection | 1 | walked | Candidate Zones for AS112 Redirection. States that every zone listed in RFC 6303 is a candidate, and closes by saying the document 'is simply concerned with provision of the AS112 redirection service and does not specify that any particular AS112 redirection be put in place'. Its one site, 5:1, is the sentence explaining why any zone owner can use the mechanism, excluded below. Choosing a zone to redirect binds that zone's owner, and the candidate list belongs to RFC 6303. |
| `6` | DNAME Deployment Considerations | 2 | walked | DNAME Deployment Considerations. Reviews whether DNAME is deployed widely enough for the mechanism to work, with Appendix A as the measurement. States that DNAME support on the AUTHORITATIVE server is a prerequisite for DNAME redirection, that a resolver without DNAME accepts and caches the synthesised CNAME RFC 6672 supplies, and that negative caching of the CNAME target follows EMPTY.AS112.ARPA's own parameters (RFC 2308), so every redirected name is negatively cached alike. Two sites: 6:1 binds a DNSSEC-validating resolver and is excluded below; 6:2 is the section's last sentence and is mapped to RFC7535-6-1, the only MUST-level id this summary declares. |
| `7` | IAB Statement Regarding This .ARPA Request | 0 | walked | IAB Statement Regarding This .ARPA Request. Records that the IAB approves the delegation of AS112 in the ARPA domain and has asked IANA to provision AS112.ARPA under RFC 3172, while taking no architectural or technical position on the specification. It binds the IAB and the IANA, and states no protocol behavior. |
| `8` | IANA Considerations, the parent heading | 0 | skipped (iana) | IANA Considerations, the parent heading. |
| `8.1` | Address Assignment | 0 | skipped (iana) | Address Assignment. The IANA registrations of 192.31.196.0/24 as AS112-v4 and 2001:4:112::/48 as AS112-v6 in the IPv4 and IPv6 Special-Purpose Address Registries [RFC6890]. IANA actions. The two prefixes themselves are what an AS112 node announces, which is RFC7535-3.1-1 on section 3.1. |
| `8.2` | Hosting of AS112.ARPA | 0 | skipped (iana) | Hosting of AS112.ARPA. States that the IANA hosts and signs the AS112.ARPA zone with infrastructure of its choosing and may adjust the SOA RDATA, and gives the zone content: the IANA-SERVERS.NET NS set, the A and AAAA of BLACKHOLE, and the NS delegations of HOSTNAME and EMPTY to BLACKHOLE. It binds the IANA. An AS112 node operator hosts the delegated EMPTY.AS112.ARPA zone, never this parent. |
| `8.3` | Delegation of AS112.ARPA | 0 | skipped (iana) | Delegation of AS112.ARPA. Records that the IANA has arranged the delegation from the ARPA zone by normal procedure and that the whois contact is specified by the IAB under RFC 3172. An IANA action. |
| `9` | Security Considerations | 0 | walked | Security Considerations. Two sentences: this document presents no known additional security concerns to the Internet, and RFC 7534 holds the security considerations for AS112 service in general. No countermeasure is directed at a nameserver, and the considerations it points at are walked in rfc/extraction/rfc7534.json section 8. |
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
