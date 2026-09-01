# RFC 7534 - AS112 Nameserver Operations

Supported. Every requirement this repository extracted from RFC 7534, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.7% | 2 of 3 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 33.3% | 1 of 3 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 3 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 5 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 3 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 16 |
| Gated MUST-level | 3 |
| Obligations that bind Ze | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 5 |
| Tagged units | 5 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7534.md` |
| Requirement shard | `rfc/requirements/rfc7534.md` |
| RFC text | `rfc/full/rfc7534.txt` |

## Enrolment

Enrolled: AS112 Nameserver Operations: three MUSTs over ze's authoritative AS112 DNS server (internal/plugins/as112, answerQuestions/zones.go). RFC7534-3.5-1 (answer authoritatively for each delegated zone) both polarities: a query at the apex of a Direct-Delegation reverse zone is answered from that zone (TestZoneAnswer_ReverseZoneNoData) while a name outside every served zone draws REFUSED with AA clear, so the node makes no authority claim over it (TestZoneAnswer_OutOfZoneRefused). RFC7534-3.5-2 (no records beyond SOA and NS) both polarities: a PTR query at the apex yields NODATA (TestZoneAnswer_ReverseZoneNoData) while the zone does contain its SOA + the two blackhole NS records (TestSOA_RFCMandatedParameters). RFC7534-3.5-3 (RFC 1918 records not hosted) is {single-polarity: positive}: the zones hold their records at the apex and nothing below it, so an RFC 1918 reverse PTR yields NXDOMAIN with no PTR in the reply and no such record is ever hosted (TestZoneAnswer_ResponseCodeByNamePosition). Ledger row unchanged (no gap).

## What the public ledger says

**Status:** Supported

**What the ledger says is covered:**

Authoritative sink for misdirected RFC 1918 and link-local reverse DNS.

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated instead of tested | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC7534-3.5-1`](#rfc7534-3.5-1), [`RFC7534-3.5-2`](#rfc7534-3.5-2)

**Annotated instead of tested (1):** [`RFC7534-3.5-3`](#rfc7534-3.5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7534-3.5-1` | AS112 nameservers answer authoritatively for each zone delegated to them (§3.5, also restated in RFC 7535 §1) | MUST | 3.5 - DNS Software | **positive:** `unit/verify` [`TestZoneAnswer_ReverseZoneNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L85). **negative:** `unit/verify` [`TestZoneAnswer_OutOfZoneRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L169) |
| `RFC7534-3.5-2` | Direct Delegation zones contain no resource records beyond SOA and NS (§3.5, db.dd-empty) | MUST | 3.5 - DNS Software | **positive:** `unit/verify` [`TestZoneAnswer_ReverseZoneNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L88). **negative:** `unit/verify` [`TestSOA_RFCMandatedParameters`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L293) |
| `RFC7534-3.5-3` | Records relating to RFC 1918 resources within the hosting site are not hosted on the AS112 nameserver itself (§3.5) | MUST | 3.5 - DNS Software | **positive:** `unit/verify` [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L247). **negative:** no negative test. **{single-polarity}:** ze's AS112 nameserver serves only empty SOA/NS Direct-Delegation zones (internal/plugins/as112/zones.go), which hold their records at the apex and nothing below it, so every RFC 1918 reverse name draws a name error and no RFC 1918 record is ever hosted -- there is no "RFC 1918 record is hosted" case to assert as a negative. The positive (an RFC 1918 reverse PTR yields NXDOMAIN, with no PTR anywhere in the reply) is proven in TestZoneAnswer_ResponseCodeByNamePosition |
| `RFC7534-3.3-1` | The chosen platform supports cloned loopback interfaces or multiple addresses on one loopback interface (§3.3) | SHOULD | 3.3 - Operating System and Host Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.3-2` | A host running an AS112 node is dedicated to that purpose, not shared with other services (§3.3) | SHOULD | 3.3 - Operating System and Host Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.3-3` | Startup order is: loopback interface configuration, then DNS software, then routing software (§3.3) | SHOULD | 3.3 - Operating System and Host Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.3-4` | The AS112 service prefix is not advertised while anycast addresses are unconfigured or DNS software is not running (§3.3) | SHOULD | 3.3 - Operating System and Host Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.4-1` | Outbound BGP advertisement is restricted by a prefix filter permitting only the AS112 service prefixes (§3.4) | SHOULD | 3.4 - Routing Software | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.4-2` | Outbound BGP advertisement is restricted by an AS_PATH filter matching only locally-originated routes (§3.4) | SHOULD | 3.4 - Routing Software | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.5-4` | AS112 nodes run as authoritative-only DNS servers with recursion disabled (§3.5) | SHOULD | 3.5 - DNS Software | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.5-5` | HOSTNAME.AS112.NET / HOSTNAME.AS112.ARPA TXT responses fit within a 512-octet UDP datagram without requiring EDNS0 (§3.5) | SHOULD | 3.5 - DNS Software | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-4.1-1` | AS112 nodes are monitored as a production service (§4.1) | SHOULD | 4.1 - Monitoring | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-4.2-1` | An AS112 node going off-line for maintenance withdraws its service-prefix BGP announcement first (§4.2) | SHOULD | 4.2 - Downtime | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-4.3-1` | Usage is measured for long-term trend and anomaly tracking (§4.3) | SHOULD | 4.3 - Statistics and Measurement | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.2-1` | New AS112 node operators notify the local community (e.g. IXP mailing list) before installation; globally-reachable nodes coordinate with other AS112 operators (§3.2, §5) | SHOULD | 3.2 - Topological Location | **positive:** no positive test. **negative:** no negative test |
| `RFC7534-3.4-3` | An IPv4-only node configures only the IPv4 anycast addresses; an IPv6-only node configures only the IPv6 anycast addresses (§3.4) | MAY | 3.4 - Routing Software | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7534 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7534-3.5-1`](#rfc7534-3.5-1)

AS112 nameservers answer authoritatively for each zone delegated to them (§3.5, also restated in RFC 7535 §1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestZoneAnswer_OutOfZoneRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L169) | unit/verify | unproven |
| positive | [`TestZoneAnswer_ReverseZoneNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L85) | unit/verify | unproven |

### [`RFC7534-3.5-2`](#rfc7534-3.5-2)

Direct Delegation zones contain no resource records beyond SOA and NS (§3.5, db.dd-empty)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSOA_RFCMandatedParameters`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L293) | unit/verify | unproven |
| positive | [`TestZoneAnswer_ReverseZoneNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L88) | unit/verify | unproven |

### [`RFC7534-3.5-3`](#rfc7534-3.5-3)

Records relating to RFC 1918 resources within the hosting site are not hosted on the AS112 nameserver itself (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L247) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc7534 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc7534.txt |
| Source fingerprint | e49b3f8cd69c5bc3 |
| Record | rfc/extraction/rfc7534.json |
| Mapped sentences | 0 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | walked | Title block, Abstract, Status of This Memo, Copyright Notice and Table of Contents. Walked rather than skipped because the site scan attributes two sites here, both excluded below: the Abstract's last paragraph and the IETF Trust Legal Provisions boilerplate. The Abstract restates section 1 and states that the document obsoletes RFC 6304. Nothing before section 1 binds a nameserver or its operator. |
| `1` | Introduction | 0 | walked | Introduction. Indicative throughout: private-use addresses are not globally unique, devices originate reverse lookups for them, those queries are answered by nobody usefully, and the AS112 project is a distributed sink for them. Its one guidance sentence, 'it is good practice for site administrators to ensure that such queries are answered locally [RFC6303]', binds the site administrator whose resolver leaks the query, not the AS112 node; rfc/short/rfc7534.md carries it under Security Considerations and RFC 6303 owns it. The last paragraph states the project's operating shape, which the AS112 plugin implements: a nameserver configured to answer authoritatively for a nominated zone set, one node in an anycast cloud. |
| `2` | AS112 DNS Service, the parent heading | 0 | walked | AS112 DNS Service, the parent heading. No text of its own. |
| `2.1` | Approach, the parent heading of 2.1.1 and 2.1.2 | 0 | walked | Approach, the parent heading of 2.1.1 and 2.1.2. No text of its own. |
| `2.1.1` | Direct Delegation | 0 | walked | Direct Delegation. Indicative: zones whose traffic should be sunk are delegated directly to AS112 nameservers, and each node is manually configured to answer for them. The delegation half binds the parent zone's operator and IANA, not the node. The node half is the same obligation as RFC7534-3.5-1, declared unsourced on 3.5. |
| `2.1.2` | DNAME Redirection | 0 | walked | DNAME Redirection. Reports that RFC 7535 defines a different approach, in which queries are redirected to an empty zone also hosted on AS112 servers using DNAME [RFC6672], and that this document introduces the capability. The obligations of that approach belong to RFC 7535, which has its own summary and its own sign-off; nothing here directs a node beyond serving EMPTY.AS112.ARPA, which 2.2 lists and RFC7534-3.5-1 covers. |
| `2.2` | Zones | 0 | walked | Zones. The zone list an AS112 nameserver answers authoritatively for: 10.IN-ADDR.ARPA, the sixteen 172.16/12 zones, 168.192.IN-ADDR.ARPA, 254.169.IN-ADDR.ARPA, EMPTY.AS112.ARPA, and the two identification zones HOSTNAME.AS112.NET and HOSTNAME.AS112.ARPA. It is a list of names, not a directive; the obligation to answer authoritatively for each of them is RFC7534-3.5-1, declared unsourced on 3.5. The section closes by pointing at 3.5 for the recommended contents. Ze's table is buildServedZones in internal/plugins/as112/zones.go, twenty-two zones with the same membership. |
| `2.3` | Nameservers | 0 | walked | Nameservers. States which nameserver names and anycast addresses the zones are delegated to (BLACKHOLE-1 and BLACKHOLE-2.IANA.ORG, PRISONER.IANA.ORG as the SOA MNAME, BLACKHOLE.AS112.ARPA for DNAME redirection), and which prefixes cover them. The delegation binds IANA and the parent zone operators. The addresses a node configures are the subject of 3.3 and 3.4 and of RFC7534-3.4-3; the names a node puts in its own SOA and NS records are the subject of 3.5. Nothing is directed at a speaker here that is not directed again there. Ze's constants are anycastV4DirectDelegationAddr and its three siblings in internal/plugins/as112/register.go. |
| `3` | Installation of a New Node, the parent heading | 0 | walked | Installation of a New Node, the parent heading. No text of its own. |
| `3.1` | Useful Background Knowledge | 0 | walked | Useful Background Knowledge. Says installation is straightforward and that experience in BGP, DNS authoritative server operations and anycast distribution may prove useful. Advice to a person, directing no software behavior. |
| `3.2` | Topological Location | 0 | walked | Topological Location. Says a node may be located anywhere, that an Internet Exchange Point is often advantageous, and that a node may advertise its service prefix for local or for global use. Its one guidance sentence, 'It is good operational practice to notify the community of users that may fall within the reach of a new AS112 node before it is installed', with 'coordination with other AS112 operators is highly recommended' for globally reachable nodes, binds the operator installing the node and is repeated in section 5. rfc/short/rfc7534.md declares it as RFC7534-3.2-1 at [SHOULD] and cites both sections; it is declared unsourced here because the modal is the lowercase 'recommended' inside 'is highly recommended', which the site scan does not attribute to this section. |
| `3.3` | Operating System and Host Considerations | 0 | walked | Operating System and Host Considerations. Four lowercase-'should' sentences, all four declared unsourced here because the site scan attributes no site to this section: the chosen platform 'should include either support for cloned loopback interfaces or the capability to bind multiple addresses to a single loopback interface' (RFC7534-3.3-1); a host acting as an AS112 node 'should be dedicated to that purpose and should not be used to simultaneously provide other services' (RFC7534-3.3-2); startup order 'should be arranged such that routing software startup follows DNS software startup, and DNS software startup follows loopback interface configuration' (RFC7534-3.3-3); and 'Wrapper scripts or other arrangements should be employed to ensure that the anycast service prefix for AS112 is not advertised while either the anycast addresses are not configured or the DNS software is not running' (RFC7534-3.3-4). The first three bind the operator choosing and building the host. The fourth lands on ze, which is both the DNS server and the BGP speaker: as112Producer.onServingChanged and withdraw (internal/plugins/as112/redistribute.go) hold the covering prefixes back until the listeners are serving and withdraw them when they stop, so the wrapper the RFC asks an operator to write is inside the product. |
| `3.4` | Routing Software | 1 | walked | Routing Software. States that each AS112 node is a BGP speaker announcing 192.175.48.0/24 and 2620:4f:8000::/48 with origin AS 112, that the examples are based on the Quagga Routing Suite, and that an IPv4-only node need not configure the IPv6 elements and an IPv6-only node need not configure the IPv4 ones (RFC7534-3.4-3, [MAY]). Most of the section is an annotated bgpd.conf, and the guidance it carries is stated in the paragraph after it: 'two restrictions on what the AS112 should advertise to its BGP neighbours: a prefix filter that permits only the service prefixes, and an AS_PATH filter that matches only locally originated routes', which rfc/short/rfc7534.md declares as RFC7534-3.4-1 and RFC7534-3.4-2. All three ids are declared unsourced here: the one site the scan attributes to this section is the zebra.conf sentence, which is about Quagga and is excluded below. Ze's covering prefixes and origin AS are coveringPrefixesFor and as112DefaultASN (internal/plugins/as112/redistribute.go, config.go). |
| `3.5` | DNS Software | 0 | walked | DNS Software. The section that carries every MUST-level id this summary declares, and not one of them is stated with a modal the site scan can see, so all five ids read from here are declared unsourced. 'AS112 nodes operate as fully functional and standards-compliant DNS authoritative servers [RFC1034]' is RFC7534-3.5-1, with the named.conf note 'the nameserver is configured to act as an authoritative-only server (i.e., recursion is disabled)' as RFC7534-3.5-4. The zone content rules are comments inside db.dd-empty and db.dr-empty: 'There should be no other resource records included in this zone' is RFC7534-3.5-2, and 'Records that relate to RFC 1918-numbered resources within the site hosting this AS112 node should not be hosted on this nameserver' is RFC7534-3.5-3. 'the responses to the queries "HOSTNAME.AS112.NET IN TXT" and "HOSTNAME.AS112.ARPA IN TXT" should fit within a 512-octet DNS/UDP datagram: i.e., it should be available over UDP transport without requiring EDNS0 support by the client' is RFC7534-3.5-5. The SOA field values, the NS targets and the $TTL of 1W are sample zone data rather than sentences, and rfc/short/rfc7534.md carries them in its DNS Software Requirements prose; ze's constants are soaRefresh, soaRetry, soaExpire, soaMinTTL and zoneTTL in internal/plugins/as112/zones.go, and the answer synthesis is zoneAnswer in the same file. Two further sentences state no obligation ze is left owing. 'The optional LOC record [RFC1876] included in each zone apex provides information about the geospatial location of the node' is offered as optional and no id declares it; ze synthesizes SOA, NS and TXT at the apex and no LOC. 'Where software implementations support it, operational data should also be carried using NSID [RFC5001]' is conditioned on the DNS software supporting NSID, and ze's DNS harness (internal/core/dnsserver) reads the EDNS0 OPT record for client-subnet (RFC 7871) and implements no NSID option, so the condition does not hold and the sentence binds nothing here. |
| `3.6` | Testing a Newly Installed Node | 0 | walked | Testing a Newly Installed Node. A worked 'dig' invocation against hostname.as112.net TXT, how to read a response that names a different node, and the note that a proper test set queries from many places over both UDP and TCP against all three anycast addresses. Instructions to the person commissioning the node, directing no behavior of the node itself. Ze's equivalent is the operator-facing as112 health command, handleAS112Health in internal/plugins/as112/health.go. |
| `4` | Operations, the parent heading | 0 | walked | Operations, the parent heading. No text of its own. |
| `4.1` | Monitoring | 0 | walked | Monitoring. One lowercase-'should' sentence, declared unsourced here: 'AS112 nodes should be monitored to ensure that they are functioning correctly, just as with any other production service.' It binds the node's operator, and the rest of the section says why: a node that answers wrongly causes failures and timeouts in dependent systems that are hard to diagnose. |
| `4.2` | Downtime | 0 | walked | Downtime. One lowercase-'should' sentence, declared unsourced here: 'An AS112 node that needs to go off-line (e.g., for planned maintenance or as part of the diagnosis of some problem) should stop advertising the AS112 service prefixes to its BGP peers', by shutting the routing software down or by withdrawing the route. The second paragraph gives the reason: an announced prefix with a dead nameserver blackholes the query traffic. The obligation binds the operator taking the node down, and ze gives them the mechanism: as112Producer.withdraw (internal/plugins/as112/redistribute.go) withdraws the covering prefixes when the listeners stop serving. |
| `4.3` | Statistics and Measurement | 0 | walked | Statistics and Measurement. One lowercase-'should' sentence, declared unsourced here: 'Use of the AS112 node should be measured in order to track long-term trends, identify anomalous conditions, and ensure that the configuration of the AS112 node is sufficient to handle the query load.' It binds the operator. The rest names free monitoring tools (bindgraph, dnstop, DSC) and suggests joining DNS-OARC collection events, which is a recommendation about a community activity and directs no software. Ze publishes the counters an operator measures with: buildMetrics in internal/plugins/as112/metrics.go. |
| `5` | Communications | 0 | walked | Communications. Repeats 3.2's notify-the-community guidance, names the as112-ops@lists.dns-oarc.net mailing list and www.as112.net, says information about a node should also be published in the DNS through the HOSTNAME.AS112.* zones (which is 3.5's TXT content, RFC7534-3.5-5), and says operators should be aware of RFC 6305 and direct site administrators appropriately. Every obligation binds the node's operator and is already declared: RFC7534-3.2-1 cites this section beside 3.2, and it is declared unsourced on 3.2 rather than twice. |
| `6` | On the Future of AS112 Nodes | 0 | walked | On the Future of AS112 Nodes. Indicative and predictive: recursive-nameserver operators are recommended by RFC 6303 to answer these zones locally, no trend toward that compliance is observable, vendors are expected to ship such defaults, the query load will not fall to zero, and the community might one day recommend retiring Direct Delegation service -- which the section says explicitly this document does not recommend. The one recommendation named belongs to RFC 6303 and binds a recursive resolver, not an AS112 node. |
| `7` | IANA Considerations, the parent heading | 0 | skipped (iana) | IANA Considerations, the parent heading. |
| `7.1` | IANA Considerations, General | 0 | skipped (iana) | IANA Considerations, General. Records who assigned the AS number and the four prefixes (ARIN for 112, 192.175.48.0/24 and 2620:4f:8000::/48; IANA for 192.31.196.0/24 and 2001:4:112::/48) and that the anycast infrastructure itself is not an IANA function. Binds IANA and the registries. |
| `7.2` | IANA Actions, the parent heading | 0 | skipped (iana) | IANA Actions, the parent heading. |
| `7.2.1` | not stated | 0 | skipped (iana) | The AAAA records IANA has added for PRISONER, BLACKHOLE-1 and BLACKHOLE-2.IANA.ORG. An IANA action on the IANA.ORG zone. |
| `7.2.2` | not stated | 0 | skipped (iana) | The registration of AS 112 in the Special-Purpose AS Numbers registry [RFC7249]. An IANA action. |
| `7.2.3` | not stated | 0 | skipped (iana) | The registration of 192.175.48.0/24 in the IANA IPv4 Special-Purpose Address Registry [RFC6890]. An IANA action. |
| `7.2.4` | not stated | 0 | skipped (iana) | The registration of 2620:4f:8000::/48 in the IANA IPv6 Special-Purpose Address Registry [RFC6890]. An IANA action. |
| `8` | Security Considerations | 0 | walked | Security Considerations. Five paragraphs, none of which directs the node's DNS or BGP behavior. Hosts should never normally send these queries and should answer them locally, which binds the querying site. Responses are unexpected and can trigger intrusion detection, so AS112 operators should expect to be contacted, with RFC 6305 as the advice to give. A loosely coordinated deployment makes a compromised node hard to detect, so operators should protect their nodes as they would any production nameserver, with RFC 2870 named as instructive: that binds the operator. The last paragraph states as fact that AS112 zones are not DNSSEC-signed, because the key material would have to be effectively public; it is a statement of what the service does not do, not an obligation. rfc/short/rfc7534.md carries all five under Security Considerations and declares no id for them. |
| `9` | References, the parent heading | 0 | skipped (references) | References, the parent heading. |
| `9.1` | not stated | 0 | skipped (references) | Normative References: RFC 1034, RFC 1918, RFC 2870, RFC 4033, RFC 4271, RFC 4786 and RFC 7535. |
| `9.2` | not stated | 0 | skipped (references) | Informative References: RFC 1876, RFC 5001, RFC 5855, RFC 6303, RFC 6304, RFC 6305, RFC 6672, RFC 6890 and RFC 7249. |
| `A` | Appendix A, A Brief History of AS112 | 0 | skipped (appendix-non-normative) | Appendix A, A Brief History of AS112. Narrative: RFC 1918 use spread from 1996, offloading IN-ADDR.ARPA from the root servers was proposed by Bill Manning and John Brown, ARIN provided the prefix and AS 112, the first nodes were deployed in 2002, and IN-ADDR.ARPA was redelegated in 2011. It directs nobody. |
| `B` | not stated | 0 | skipped (appendix-non-normative) | Appendix B, Changes since RFC 6304: IPv6 transport, DNAME-based delegation of additional zones, the consequent configuration guidance, clarified information-leak text, and the direction to IANA to register the prefixes. A change list over the obsoleted document. The derivation attributes the trailing Acknowledgements and Authors' Addresses to this section, and neither binds anybody either. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Abstract's closing paragraph, non-normative by RFC convention. Its lowercase 'required' is inside 'the steps required to install a new AS112 node', which names what the document is about rather than obliging anybody to do anything. The installation guidance itself is section 3, and every id read from it is declared unsourced on the section that states it. | This document describes the steps required to install a new AS112 node and offers advice relating to such a node's operation. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the IETF Trust Legal Provisions paragraph of the Copyright Notice. Its lowercase 'must' binds a party extracting Code Components from the document text, and directs no DNS or BGP behavior. The document's sample bgpd.conf, named.conf and zone files are Quagga and BIND9 configuration, not code components ze reuses: internal/plugins/as112 synthesizes the same zone contents in Go. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `3.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A description of another system, and the extractor cannot see which: 'zebra.conf' is a file of the Quagga Routing Suite, which the same section names as the basis of its examples while saying 'other software packages exist that also provide suitable BGP support for AS112 nodes'. The sentence states that Quagga needs a zebra daemon to reach the kernel FIB, so its lowercase 'required' is a fact about that package's architecture and not an obligation on an AS112 node. Ze is a single daemon with its own FIB path and no zebra; the two restrictions this section does place on an AS112 node's BGP advertisement are RFC7534-3.4-1 and RFC7534-3.4-2, declared unsourced on 3.4. | The "zebra.conf" file is required to provide integration between protocol daemons (bgpd, in this case) and the kernel. |

## Superseded

No document obsoletes RFC 7534, so its obligations are stated where they were written.
