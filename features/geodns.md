# GeoDNS

## Meta

| Field | Value |
|-------|-------|
| Name | GeoDNS |
| Page | docs/guide/as112.md |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/geodns |
| Real-path tests | test/plugin/geodns-show.ci, test/plugin/geodns-dot-pki.ci, test/ui/doctor-geodns.ci, test/parse/geodns-config.ci |
| RFCs | rfc7871, rfc7858, rfc8484 |
| Docs | docs/guide/as112.md |
| Doc review | 2026-10-07: every source anchor resolves (answerQuestions, ze-geodns-conf.yang client-subnet and tls certificate) |
| Defect review | 2026-10-07: journal guard-added-to-one-half-of-a-pair.md row names geodns; journal rows naming a Component, not each re-verified here: gate-verdict-depends-on-the-machine.md:119, guard-added-to-one-half-of-a-pair.md:32, guard-added-to-one-half-of-a-pair.md:33, guard-added-to-one-half-of-a-pair.md:35, guard-added-to-one-half-of-a-pair.md:47 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Authoritative DNS server that selects the answer from the client's source IP, taken from the EDNS0 client-subnet option (RFC 7871) or the connecting address. Zones, host sets, per-source selection, and a configurable SOA (contact, serial mode, refresh, retry, expire, minimum) with the serial recomputed per generation. Optional DoT (RFC 7858) and DoH (RFC 8484), whose certificate is either a `cert-file`/`key-file` pair or a `tls { certificate <name> }` reference into the `pki {}` store; the two forms are mutually exclusive. Shares the answer-policy harness described below with `as112`. <!-- source: internal/plugins/geodns/server.go -- answerQuestions, per-source resolution --> <!-- source: internal/plugins/geodns/yang/ze-geodns-conf.yang -- zones, client-subnet, tls certificate -->
