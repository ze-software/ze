# A test still asserts behaviour the code was deliberately changed away from

The producer was changed on purpose, its tests were not, so the suite stays red
against correct code and hides any real failure in the same package.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | reactor unit tests, e.g. `TestRunEgressPolicyChainNoExportFiltersAccepts`, `TestEgressFilterRejectWithdrawsOnBothRails` | At 7c45216f1e, tagged or not, 116 reactor tests fail; sampled ones expect an EBGP peer with no export policy to accept, which RFC 8212 Section 3 default-reject refuses. The plugin modify and reload tests losing egress routes likely share the cause | fixed: bebc0d9cf2 gives non-RFC fixtures an accepting policy, 1ccb8dca31 renames the egress test to the reject it asserts; both named tests green |
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | reactor RS tests (`TestReactorForwardRSBasic`, `TestReactorForwardRSFallback`, two RFC 2545 RS tests) | Red at a clean HEAD: the RS fast path refuses, the peer lacks receive `update-received`, so nothing is dispatched; the fixtures predate that grant | fixed: c8a7d38c3f fixes the RS fixtures; TestReactorForwardRSBasic and RSFallback green |
| 2026-10-31 | bgp-rfc8212-ebgp-default-reject (found, not caused) | test/plugin: adj-rib-in-query, adj-rib-in-replay-{addpath-source,on-peerup,unowned-peer}, redistribute-{as112-origin-ebgp,bgp-to-ospf-no-plumbing,block-does-not-gate-peer-routes,no-block-peer-route-reaches-rib,export-modify,export-reject} | Red at a049380934: EBGP peer, no import policy, so the RFC 8212 default reject from ff2aac12b0 (`runIngressPolicyChain`) kept the route out of Adj-RIB-In | fixed: ca21d0b884, e6bcdec054, f8f7628bdf add bgp-level import/export `final accept`, as c8a7d38c3f did; 10/10 green at 6b1063fe8b |
