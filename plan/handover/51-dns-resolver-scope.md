# Handover: DNS scope under the home-resolver rules (2026-11-19)

Owner rules committed in 76dc8b66bc (ai/META-DECISIONS.md, "Goal and scope"):
DNS zone transfer is not required; a DNS requirement that binds only a role
neither the home-connection resolver nor GeoDNS/AS112 fill is out of scope,
its annotation naming that role; Ze may build its own DNS library on
miekg/dns ahead of use, counting a requirement met only once a shipped path
uses it.

## Outstanding items

| # | Item | State |
|---|------|-------|
| 1 | RFC gate cannot express the role annotation for RFC 1035 | Gate change written (2026-11-19), uncommitted until the main thread runs its owed commands, see below |
| 2 | RFC1035-4.2-1 (zone refresh over TCP) annotated as binding the secondary server role | Meta `Ze roles` row added to rfc/short/rfc1035.md; the annotation, in the role-sentence form, is held in "Item 2" below and goes into the summary in the commit after the gate commit, because HEAD's `le` refuses it |
| 3 | RFC 4035 rows 3.1.5-3 to 3.1.5-7 recorded as `{gap}` for zone-transfer MUSTs | No exclusion shape fits (see "Item 3" below). The `{gap}` reasons now name the primary or secondary zone-transfer server role and the owner rule; the rows stay counted gaps until the main thread picks a route |
| 4 | RFC1035-3.3.13-1 (SOA MINIMUM as TTL floor, redefined by RFC 2308 Section 4) | A resolver does negative caching, so RFC 2308 is resolver work: enrol RFC 2308 under the fttx-edge use case (summary, extraction walk, tests), then mark 3.3.13-1 updated by it |
| 5 | Sweep every DNS summary (rfc/short with Support dns: 1035, 1877, 2181, 2782, 4033, 4034, 4035, 5625, 7084, 7534, 7858, 7535, 8106, 7871, 8484 and others) against the resolver role | Not started |
| 6 | rfc/extraction/rfc1035.json reasons cite "ze's only DNS code, the authoritative answerQuery"; internal/plugins/dnsresolver and internal/core/dnsserver Recursive now exist, so some querier/resolver `binds-another-role` exclusions (4.2.1:1, the Section 7 sites) may bind a role Ze fills | Re-walk those sites |
| 7 | rfc/short/rfc1035.md Enrolment reason still says the owner ruled RFC 1035 out of scope on 2026-08-18, while Use cases is fttx-edge | Correct the reason |
| 8 | RFC 9582 (ROA validation, 16 rows): not DNS; the owner has not said whether ROA validation is Ze's role or the relying party's | Owner question, carry it |

## Item 1: the gate change

`./le rfc check` refuses every form of the annotation for RFC 1035:

- Quoted `{not-applicable: "<quote>"; binds <role>; <why>}` must match shape (a), the
  row's own sentence with an upper-case BCP 14 keyword, or shape (b), a keyword-less
  sentence with a scoping marker that names the role
  (`internal/le/rfc/summary.go::parseNotApplicable`, `notapplicable_roles.go::boundRoleRefusal`,
  `scopingRefusal`). `firstKeyword` is case-sensitive (`check_evidence.go`), and RFC 1035
  (1987) has only lowercase "must", so (a) never matches; no Section 4.2 sentence has
  both a marker and a role, so (b) never matches. A mismatch is a parse error.
- Unquoted `{not-applicable: reason}` parses but `(*evidenceReader).gap` reports "carries
  no role-condition quote", and `checkExclusionEvidence` refuses a touched row; the
  backlog count in rfc/exclusion-evidence-backlog.txt may only fall.

Done (2026-11-19, uncommitted until the owed commands pass). The register of a
document's keywords is one derived fact, the RFC number in its stem below 2119
(`internal/le/rfc/keyword_register.go::keywordRegisterOf`). Every keyword reader goes
through it: `firstKeyword`, `firstMandatoryKeyword` and `levelsIn` are methods of
`keywordRegister`, so shape (a), `ownObligationRefusal`, `levelKeywordAt`, the
`{feature-declined}` optionality and conditional readers and the `{intent}` quote all
read a pre-RFC 2119 "must" as MUST and "must not" as MUST NOT. Upper-case reading is
unchanged for later RFCs and drafts, and lowercase "should" and "may" stay prose.

Shape (a) with lowercase "must" did not suffice alone: the subject of the RFC 1035
Section 4.2 sentence is "Zone refresh activities", not a role, so `subjectRefusal`
refuses `binds a secondary server`. The second relaxation was therefore added, for a
pre-RFC 2119 RFC only: `binds <role>; §<section> "<sentence naming the role>"; <why>`
(`summary.go::parseRoleSentence`, `notapplicable_roles.go::roleSentenceRefusal`,
`check_evidence.go::roleSentenceGap`). Tests: `keyword_register_test.go`. Docs:
docs/contributing/rfc-conformance-gates.md, the `{not-applicable}` row and the
Optionality row.

## Item 2: the annotation, ready once item 1 is committed

The Meta row `| Ze roles | name server, resolver |` is in rfc/short/rfc1035.md. The
annotation is NOT in the summary yet: the shared `le` is built from HEAD, and HEAD's
gate refuses this form, so in the working tree it makes rfc1035.md fail to parse for
every session (the derived-file hook already refused on it). Append it to the
RFC1035-4.2-1 row, after `(§4.2)` and one space, in the commit after the gate commit.
The role sentence moved from a parenthesis in the reason to the `§2.2 "..."` segment
the gate now reads after `binds a secondary server`; the rest of the reason is as
drafted:

```
{not-applicable: "Zone refresh activities must use virtual circuits because of the need for reliable transfer."; binds a secondary server; §2.2 "Designated secondary servers can acquire zones and check for updates from the primary server using the zone transfer protocol of the DNS."; zone refresh is the zone transfer a secondary server pulls from the primary server, and per the owner rule in ai/META-DECISIONS.md (DNS zone transfer is not required) no part of Ze fills the primary or secondary zone-transfer server role: internal/plugins/geodns/server.go::answerQuestions sends an AXFR or IXFR qtype to its default arm (SOA in Authority, NODATA) and internal/plugins/as112/zones.go::answerQuestions does the same, so neither serves a zone; internal/core/dnsserver/handler.go::Authoritative answers a non-QUERY opcode with NOTIMP; internal/plugins/dnsresolver (resolve.go::resolve) holds a cache, not a zone; grep -rniE 'axfr|ixfr|zone transfer|dns\.Transfer|OpcodeNotify' over those trees and internal/core/dnsserver finds no producer, only the geodns YANG text "it serves no AXFR"}
```

Evidence checked 2026-11-19: GeoDNS's qtype switch handles A, AAAA, SRV, ANY, SOA, NS
and sends the rest to `default:` (SOA in Authority); AS112 handles SOA, NS, TXT, default
NODATA; the shared handler refuses a non-QUERY opcode with NOTIMP; the resolver forwards
`q.Qtype` (resolve.go) and holds no zone. No other RFC 1035 row binds only the
zone-transfer role (3.3.13-2 binds the server building the response).

## Item 3: why no shape fits RFC 4035 Section 3.1.5

RFC 4035 (2005) is post-RFC 2119, so the role-sentence form is refused there, and
neither shape it keeps fits rows 3.1.5-3 to 3.1.5-7:

- Shape (a): each sentence's subject is a record ("the DS RRset", "NSEC RRs", "The
  parental NSEC RR at a zone cut", "the NSEC at the zone apex of the child zone",
  "RRSIG RRs"), never a role.
- Shape (b): no keyword-less sentence of Section 3.1.5 carries a scoping marker. The
  only role the section names is "authoritative name server", a role Ze fills through
  GeoDNS and AS112, so `{not-applicable}` on it is barred by rfc-compliance.
- RFC 4035 names no primary or secondary server anywhere; the role is defined in
  RFC 1035 Section 2.2, and the role sentence must be from the row's own RFC.

The `{gap}` reasons now name the primary or secondary zone-transfer server role and
the owner rule, with the GeoDNS, AS112, shared-handler and grep evidence; the rows stay
counted gaps, published under Unmet. Two routes would take them out, both gate
decisions for the main thread: (1) extend the role-sentence form to a post-RFC 2119 RFC
and to a sentence of another RFC (here RFC 1035 Section 2.2), or (2) read the heading
"Responding to Queries for Type AXFR or IXFR" as a declined-feature heading
(`{feature-declined: heading ...}` with a `Declined features` Meta row), which needs the
owner rule to count as Thomas declining zone transfer, and "not required" is not
"never".
