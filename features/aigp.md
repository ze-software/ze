# AIGP (RFC 7311)

## Meta

| Field | Value |
|-------|-------|
| Name | AIGP (RFC 7311) |
| Kind | protocol |
| Scope | partial |
| Scope gaps | next-hop-self source-cost propagation end to end, automatic origination |
| Level | experimental |
| Components | internal/component/bgp/plugins/aigp, internal/component/bgp/plugins/rib |
| Real-path tests | test/parse/dead-capability-rejected.ci |
| RFCs | rfc7311 |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; judged at HEAD: BestStepAIGP sits in internal/component/bgp/plugins/rib/bestpath.go and capability { aigp } is refused by test/parse/dead-capability-rejected.ci |
| Defect review | 2026-10-07: judged at HEAD; journal assertion-reads-state-the-scheduler-owns (flake) taken as open; another session is editing AIGP code |
| Extra criteria | supported: configured AIGP origination reaches a peer through the daemon = test/plugin/aigp-originate.ci |

## Description

Accumulated IGP Metric path attribute: wire encoding and decoding, receive validation, structured JSON exposure with RFC 4271 attribute flags, per-session enablement (internal sessions on, external sessions off unless configured), configured origination, next-hop-self accumulation, and best-path selection after LOCAL_PREF and before AS_PATH (RFC 7311 Section 4.1). AIGP is an attribute, not a capability, so `capability { aigp }` is refused. Next-hop-self source-cost propagation is not yet established end to end, and automatic origination is not enabled. <!-- source: internal/component/bgp/plugins/aigp/ -- AIGP plugin --> <!-- source: internal/component/bgp/plugins/rib/bestpath.go -- BestStepAIGP --> <!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- container aigp -->
