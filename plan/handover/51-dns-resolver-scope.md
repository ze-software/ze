# Handover: DNS scope under the home-resolver rules (2026-11-19)

Owner rules committed in 76dc8b66bc (ai/META-DECISIONS.md, "Goal and scope"):
DNS zone transfer is not required; a DNS requirement that binds only a role
neither the home-connection resolver nor GeoDNS/AS112 fill is out of scope,
its annotation naming that role; Ze may build its own DNS library on
miekg/dns ahead of use, counting a requirement met only once a shipped path
uses it.

## State at 2026-11-20 (session 57690576 hands over here)

Landed on main: the pre-2119 keyword rule and its gate (8cc0a93795, c4b77f99df,
register derived from the extraction's declared `prose` plus no BCP 14 citation);
RFC1035-4.2-1 and RFC2181-5.5-4 annotated as binding the zone-transfer role
(c9117eb99f, 0fe775d8f2); RFC 2308 enrolled (c9117eb99f) and its met rows proven
(8f52d99804); DNS defects fixed (cd9c79b63c: DoT on 53, cleartext on 853, DoH
negative freshness, GeoDNS ECS echo and FORMERR, SRV target); RFC 2181 and RFC 1035
resolver rows proven and the RFC 1035 extraction re-walked (3406b2f346); query
retransmission (3eabbf1443, RFC1035-4.2.1-4 met). 3eabbf1443 committed the two
internal/le/site/testdata goldens whole, which also carried another session's edits;
its body does not say so.

## Outstanding items

| # | Item | State |
|---|------|-------|
| A | Uncommitted annotation batch in the working tree: rfc/short/rfc4035.md (41 signer and signed-zone rows keep `{gap}` with reasons naming the role, because no not-applicable shape fits), rfc4034.md (Enrolment reason no longer says "declined"), rfc7534.md (3.5-3 lowered to SHOULD NOT, rfc/corrections/rfc7534.md), rfc7535.md (6-1 retired, rfc/corrections/rfc7535.md), rfc7871.md and rfc8484.md (see B), rfc/extraction/rfc7535.json and rfc7871.json, features/lan-dns-resolver.md | Not checked clean. The RFC gate reported: RFC7871-7.1.2-5 names its producer as a Go method expression `(*Resolver).queryServer` (use `Resolver.queryServer`); rfc/extraction/rfc7535.json raised its exclusions from 5 to 6 with no `resign-reason` (add one, name the reviewer, bump signed-off); RFC4035-2.2-1 and 3.1.1-4 audit verdicts are stale because cd9c79b63c changed geodns answerQuestions (re-read with ze-rfc-audit). Then run the gate and commit |
| B | Owner question: the batch marks 18 resolver-side RFC 7871 ECS rows and RFC8484-5.4-2 (media types other than application/dns-message) `{feature-declined}`. The META rule says a feature leaves the count as declined only when Thomas said Ze will never build it, and he has not said so for resolver ECS or other DoH media types | Ask Thomas: never build resolver-side ECS and other DoH media types (keep declined), or later (revert those rows to `{gap}`) |
| C | RFC 4035 3.1.5-3..-7 and RFC 4034/4035 signer rows stay counted gaps: RFC 4035 adopts BCP 14 and never names the bound role in a sentence the gate reads. A cross-RFC role sentence (RFC 1035 Section 2.2 for secondary servers) would express them | Gate extension, not built: new syntax `RFC 1035 §2.2 "<sentence>"`, roleSentenceGap loading another RFC's text |
| D | RFC 7871 positives without negatives, RFC 2308 DNSSEC rows (3-2, 5-2, 5-3, 6-2) | Gaps owed with DNSSEC validation (plan/post-release/features/spec-dns-resolver-dnssec-validation.md) |
| E | Interop test for the GeoDNS ECS echo (dig +subnet or a resolver with ECS against geodns) | Owed by interop-and-goal-validation for a wire change; not written |
| F | geodns/rfc7871_server_test.go VALIDATES comments describe the old no-echo behaviour | Tagged test: judge before the edit |
| G | The stub resolver's extractRecords appends a CNAME target into the A/AAAA lists | Unverified whether a caller is hurt; trace it |
| H | RFC 7534 3.5-1 and 3.5-2 are MUST rows on indicative or lowercase sentences in a BCP 14 RFC | Fix the levels with a correction |
| I | RFC 9582 (ROA validation, 16 rows): not DNS | Owner question: ROA validation Ze's role or the relying party's |
| J | Two drafts (draft-ietf-idr-linklocal-capability, sflow-v5) write lowercase obligations but stay upper case because drafts are unchanged | Owner may rule |

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
