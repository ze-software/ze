# Authoritative DNS Answer Policy

## Meta

| Field | Value |
|-------|-------|
| Name | Authoritative DNS Answer Policy |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/core/dnsserver |
| RFCs | rfc1035 |
| Docs | docs/guide/as112.md |
| Doc review | 2026-10-07: checked Authoritative = Rcode != RcodeRefused, RcodeNotImplemented and udpReplyLimit with the 512-octet bound in internal/core/dnsserver/handler.go |
| Defect review | 2026-10-07: journal guard-added-to-one-half-of-a-pair.md row names the shared DNS handler (stale QEMU tests); journal rows naming a Component, not each re-verified here: comment-describes-superseded-behaviour.md:28, guard-added-to-one-half-of-a-pair.md:34, guard-added-to-one-half-of-a-pair.md:35, guard-added-to-one-half-of-a-pair.md:47, test-against-broken-path.md:50 |
| Extra criteria | supported: a dig-style real-path test of each answer class = test/plugin/dns-answer-policy.ci; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

One harness serves `as112` and `geodns`, so both answer a malformed or out-of-scope query the same way. A name outside every served zone gets REFUSED with the AA bit CLEAR, because RFC 1035 Section 4.1.1 gives AA one meaning and a responder that keeps it set on a Refused reply asserts an authority it does not have. A name inside a served zone that the zone does not own gets NXDOMAIN. A name that exists with no record of the requested type gets NOERROR with an empty Answer. Both negative answers carry the zone SOA in the Authority section, which RFC 2308 Section 3 requires so a resolver can cache them. A non-query opcode gets NOTIMP. A UDP reply is bounded to 512 octets (RFC 1035 Sections 2.3.4 and 4.2.1) or to the larger buffer the requestor advertised in an OPT record (RFC 6891 Section 6.2.3), with TC set; an advertisement below 512 does not lower the bound. TCP, DoT and DoH are unbounded. <!-- source: internal/core/dnsserver/handler.go -- Authoritative = Rcode != RcodeRefused, RcodeNotImplemented, udpReplyLimit --> <!-- source: internal/plugins/geodns/server.go -- buildSOA in the Authority section -->
