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
| 1 | RFC gate cannot express the role annotation for RFC 1035 | Blocked on a gate change, see below |
| 2 | RFC1035-4.2-1 (zone refresh over TCP) annotated as binding the secondary server role | Annotation text ready, below; waits on item 1 |
| 3 | RFC 4035 rows 3.1.5-3 to 3.1.5-7 recorded as `{gap}` for zone-transfer MUSTs | Contradict the new rule; re-annotate with the same shape once item 1 lands |
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

Chosen route (the owner rule requires the annotation to name the role, so keeping
`{gap}` is not an option): for a document with no RFC 2119 key-words section, let shape
(a) accept the row's own lowercase level keyword, and let the bound role come from a
role-defining sentence in another section named by a `(Section N)` locator in the
reason. Code: `notapplicable_roles.go::boundRoleRefusal` and `firstKeyword`,
`check_evidence.go::roleQuoteGap` and `ownObligationRefusal`, their tests, and
docs/contributing/rfc-conformance-gates.md (the not-applicable shapes table). Owed:
scoped tests and lint of internal/le/rfc, then `./le rfc check`.

## Item 2: the annotation, ready to apply

Also add a Meta row `| Ze roles | name server, resolver |` (both words occur in
rfc/full/rfc1035.txt, which `checkZeRolesInText` requires).

```
{not-applicable: "Zone refresh activities must use virtual circuits because of the need for reliable transfer."; binds a secondary server; zone refresh is the zone transfer a secondary server pulls from the primary server (RFC 1035 Section 2.2: "Designated secondary servers can acquire zones and check for updates from the primary server using the zone transfer protocol of the DNS."), and per the owner rule in ai/META-DECISIONS.md (DNS zone transfer is not required) no part of Ze fills the primary or secondary zone-transfer server role: internal/plugins/geodns/server.go::answerQuestions sends an AXFR or IXFR qtype to its default arm (SOA in Authority, NODATA) and internal/plugins/as112/zones.go::answerQuestions does the same, so neither serves a zone; internal/core/dnsserver/handler.go::Authoritative answers a non-QUERY opcode with NOTIMP; internal/plugins/dnsresolver (resolve.go::resolve) holds a cache, not a zone; grep -rniE 'axfr|ixfr|zone transfer|dns\.Transfer|OpcodeNotify' over those trees and internal/core/dnsserver finds no producer, only the geodns YANG text "it serves no AXFR"}
```

Evidence checked 2026-11-19: GeoDNS's qtype switch handles A, AAAA, SRV, ANY, SOA, NS
and sends the rest to `default:` (SOA in Authority); AS112 handles SOA, NS, TXT, default
NODATA; the shared handler refuses a non-QUERY opcode with NOTIMP; the resolver forwards
`q.Qtype` (resolve.go) and holds no zone. No other RFC 1035 row binds only the
zone-transfer role (3.3.13-2 binds the server building the response).
