# RFC 9774 AS_SET and AS_CONFED_SET deprecation

## Meta

| Field | Value |
|-------|-------|
| Name | RFC 9774 AS_SET and AS_CONFED_SET deprecation |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 9774 Sections 5.2 and 6.1 bind route aggregation, which Ze does not implement (feature-out-of-scope in rfc/extraction/rfc9774.json) |
| Level | experimental |
| Components | internal/component/bgp/message, internal/component/bgp/reactor |
| Real-path tests | test/plugin/rfc9774-as-set-receive-treat-as-withdraw.ci, test/encode/rfc9774-as-set-not-advertised.ci, test/plugin/rfc9774-as-set-api-not-advertised.ci |
| Interop | bgp/bgp-rfc9774-as-set-frr |
| RFCs | rfc9774 |
| Docs | docs/features/bgp-protocol.md |
| Doc review | 2026-10-19: every source anchor in the Description resolves to its file and symbol; the two docs/features/bgp-protocol.md rows match ASPathSetSegment and the writeUpdateBody guard |
| Defect review | 2026-10-19: the one defect met (static UPDATE grouping key omitting raw attributes) is fixed in f3cb2fe96f, journal row key-omits-a-fact-the-builder-uses marked fixed; no plan/immediate spec names RFC 9774 |

## Description

Ze neither accepts nor advertises an AS_SET or AS_CONFED_SET, with no configuration and no switch. A received UPDATE whose AS_PATH or AS4_PATH holds either segment type is treat-as-withdraw (RFC 7606), on every session whatever the AS width, so the route is never installed and a route the peer announced earlier is withdrawn. <!-- source: internal/component/bgp/message/rfc9774.go -- ASPathSetSegment --> <!-- source: internal/component/bgp/message/rfc7606.go -- validateASPath --> <!-- source: internal/component/bgp/message/rfc7607.go -- validateAS4PathAttr --> On send, every UPDATE a producer hands the session (relay, API, configured static route, plugin) is checked in the one writer: when any AS_PATH or AS4_PATH occurrence holds a set, the UPDATE is written as the withdrawal of its prefixes instead, logged as a rate-limited warning naming the peer and the route count, and counted on `ze_bgp_as_set_withdrawn_total`. Only the `send bgp <peer> raw` test instrument writes operator bytes as given. <!-- source: internal/component/bgp/reactor/session_rfc9774.go -- transmittedPathHoldsSet, writeUpdateAsWithdrawal --> <!-- source: internal/component/bgp/reactor/session_write.go -- writeUpdateBody -->
