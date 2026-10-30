# A test still asserts behaviour the code was deliberately changed away from

The producer was changed on purpose, its tests were not, so the suite stays red
against correct code and hides any real failure in the same package.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | reactor unit tests, e.g. `TestRunEgressPolicyChainNoExportFiltersAccepts`, `TestEgressFilterRejectWithdrawsOnBothRails` | At 7c45216f1e, tagged or not, 116 reactor tests fail; sampled ones expect an EBGP peer with no export policy to accept, which RFC 8212 Section 3 default-reject refuses. The plugin modify and reload tests losing egress routes likely share the cause | open: re-derive each red test's expectation from RFC 8212 Section 3 or give its peer an explicit policy, then rerun the package |
| 2026-10-30 | route-policy-0-umbrella (found, not caused) | reactor RS tests (`TestReactorForwardRSBasic`, `TestReactorForwardRSFallback`, two RFC 2545 RS tests) | Red at a clean HEAD: the RS fast path refuses, the peer lacks receive `update-received`, so nothing is dispatched; the fixtures predate that grant | open: give the fixtures' peers the grant the fast path requires, then rerun |
