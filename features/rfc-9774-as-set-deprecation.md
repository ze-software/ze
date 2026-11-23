# RFC 9774 AS_SET and AS_CONFED_SET deprecation

## Meta

| Field | Value |
|-------|-------|
| Name | RFC 9774 AS_SET and AS_CONFED_SET deprecation |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/message, internal/component/bgp/reactor |
| Real-path tests | test/plugin/rfc9774-as-set-receive-treat-as-withdraw.ci, test/encode/rfc9774-as-set-not-advertised.ci, test/plugin/rfc9774-as-set-api-not-advertised.ci |
| Interop | bgp/bgp-rfc9774-as-set-frr |
| RFCs | rfc9774 |
| Docs | docs/features/bgp-protocol.md |
| Doc review | 2026-11-23: the bgp aggregate section added to docs/features/bgp-protocol.md (AS_PATH the common leading AS_SEQUENCE, never an AS_SET; brief-origin-as per RFC 9774 Section 5.2) agrees with the aggregation sentence here, and the selection deferral paragraph added beside it names no AS_PATH segment, so neither contradicts this declaration; the removed Scope gap was false: aggregateBriefASPath in internal/component/bgp/plugins/rib/aggregate.go implements RFC9774-5.2-1 through brief-origin-as, rfc/short/rfc9774.md holds no gap or partial row, and rfc/extraction/rfc9774.json records Section 6.1 as an operator obligation; the added aggregation sentence matches aggregateBriefASPath, which builds the common leading AS_SEQUENCE and emits no AS_SET. Earlier, 2026-10-19: every source anchor in the Description resolves to its file and symbol; the two docs/features/bgp-protocol.md rows match ASPathSetSegment and the writeUpdateBody guard |
| Defect review | 2026-10-19: the one defect met (static UPDATE grouping key omitting raw attributes) is fixed in f3cb2fe96f, journal row key-omits-a-fact-the-builder-uses marked fixed; no plan/immediate spec names RFC 9774 |

## Description

Ze never advertises an AS_SET or AS_CONFED_SET, and by default does not accept one. A received UPDATE whose AS_PATH or AS4_PATH holds either segment type is treat-as-withdraw (RFC 7606), whatever the AS width, so the route is never installed and a route the peer announced earlier is withdrawn. The one exception is a peer whose `session as-set` the operator set to `accept` (the default is `withdraw`), the operator configuration RFC 9774 Section 3 allows; `ze exabgp migrate` writes it for an ExaBGP neighbor configured `as-set accept`. <!-- source: internal/component/bgp/reactor/config_as_set.go -- applyASSetPolicy --> <!-- source: internal/component/bgp/message/rfc9774.go -- ASPathSetSegment --> <!-- source: internal/component/bgp/message/rfc7606.go -- validateASPath --> <!-- source: internal/component/bgp/message/rfc7607.go -- validateAS4PathAttr --> On send, every UPDATE a producer hands the session (relay, API, configured static route, plugin) is checked in the one writer: when any AS_PATH or AS4_PATH occurrence holds a set, the UPDATE is written as the withdrawal of its prefixes instead, logged as a rate-limited warning naming the peer and the route count, and counted on `ze_bgp_as_set_withdrawn_total`. Only the `send bgp <peer> raw` test instrument writes operator bytes as given. <!-- source: internal/component/bgp/reactor/session_rfc9774.go -- transmittedPathHoldsSet, writeUpdateAsWithdrawal --> <!-- source: internal/component/bgp/reactor/session_write.go -- writeUpdateBody --> A route aggregate (`bgp aggregate <prefix>`) carries no AS_SET either: its AS_PATH is the contributors' common leading AS_SEQUENCE, and `brief-origin-as <asn>` truncates it after the right-most instance of that AS, the consistent brief aggregation RFC 9774 Section 5.2 asks for. <!-- source: internal/component/bgp/plugins/rib/aggregate.go -- aggregateBriefASPath -->
