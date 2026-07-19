# 1179 -- fixit-local-asn-config-key

## Context

The `role` and `gr` BGP plugins each carried a near-duplicate `extractLocalASN`
that read a config key `bgp/local-as` that the config tree NEVER produces (the
global local AS lives at `bgp/session/asn/local`). Both readers therefore
returned 0 for every real configuration and each caller treated 0 as a valid
answer. Two RFC behaviors were inert as a result: RFC 9234 R008 OTC egress
stamping (skipped by the `localASN > 0` guard) and RFC 9494 Section 4.5.3 LLGR
partial deployment (every iBGP peer misclassified as eBGP, so stale routes were
withdrawn instead of depreferenced). The two copies had also DIVERGED (gr lacked
role's `string` case and warnings), so a one-line key rename could not fix both.

## Decisions

- Chose to populate the value the plugins already receive over adding a second
  raw-tree read: the reactor fills `filterapi.PeerFilterInfo.LocalAS` on the
  forward path (`peer_forward_facts.go`), both plugins read `dest.LocalAS`, and
  BOTH `extractLocalASN` copies are DELETED (root cause removed, not renamed).
- Chose the EFFECTIVE per-peer `s.LocalAS` over the global `s.GlobalLocalAS` for
  the single `PeerFilterInfo.LocalAS` field, because (a) the readvertise rail
  already fills it from `peer.Settings().LocalAS` (reactor_api_batch.go) so both
  egress rails agree, and (b) it honors a per-peer `session/asn/local` override,
  which a captured global value gets wrong. OTC stamps `dest.LocalAS` too; absent
  an override it equals the global, so RFC 9234 R008 is satisfied.
- Rejected a shared cross-plugin reader / importing one plugin's helper into the
  other: it would violate plugin self-containment and tier rules. Reading the
  reactor-provided `filterapi` field keeps both plugins isolated.
- Fail-closed (ai/rules/fail-closed-guards.md): `dest.LocalAS == 0` now SKIPS the
  OTC stamp AND emits a WARN, so a missing local AS is observable, never a silent
  valid-looking zero. (A peer with no local AS cannot be established -- config
  parsing rejects it -- so 0 signals a wiring gap.)

## Consequences

- Any egress filter can now read our effective local AS per destination from
  `dest.LocalAS` (and `src.LocalAS`); both construction sites in the forward path
  fill it. No plugin re-parses the config JSON for the local AS.
- The masking defect that made the OTC functional test (test 390) vacuous was
  already fixed upstream (peer_contract.go `isSelfValidated` returns false when a
  check-mode ze-peer is present), so test 390 now genuinely governs and is GREEN
  for the right reason.
- AC-3 (LLGR stale iBGP -> NO_EXPORT + LOCAL_PREF=0) is exercised end-to-end by
  the pre-existing rib-arch-7 test `llgr-readvertise-multipeer.ci` (test 250),
  which passes with the fix; the deterministic guard for the classification is
  the new `TestLLGREgressIBGPClassification` unit test.

## Gotchas

- The config tree delivers YANG leaf values as JSON STRINGS (Tree.values is
  map[string]string). role's copy had a `string` case; gr's did NOT, so even a
  naive key rename in gr would still return 0 for string-delivered leaves. This
  is why the two copies could not be fixed by one identical change -- and why
  deleting both in favor of the reactor-provided field is cleaner than repairing.
- A unit test that hand-writes its input tree (`{"bgp":{"local-as":65001}}`)
  proves only that the parser reads its own fixture; that shape never occurs in
  production. Drive readers from the reactor-built tree
  (reactor `TestForwardFactsFilterInfo`, `TestPeersFromTree`).
- `test/bgp/` does not exist in this repo; BGP `.ci` functional tests live in
  `test/plugin/`. The spec's proposed `test/bgp/llgr-stale-ibgp-noexport.ci`
  would have duplicated the committed `test/plugin/llgr-readvertise-multipeer.ci`.

## Files

- internal/component/bgp/reactor/peer_forward_facts.go (fill filterInfo.LocalAS = s.LocalAS)
- internal/component/bgp/reactor/reactor_api_forward.go (fill src filterInfo.LocalAS)
- internal/component/bgp/plugins/role/otc.go (stamp dest.LocalAS; fail-closed warn)
- internal/component/bgp/plugins/role/role.go (drop filterLocalASN/getLocalASN plumbing)
- internal/component/bgp/plugins/role/config.go (delete extractLocalASN; drop imports)
- internal/component/bgp/plugins/gr/gr.go (drop localAS extraction)
- internal/component/bgp/plugins/gr/gr_egress.go (iBGP = dest.PeerAS==dest.LocalAS; drop localAS field)
- internal/component/bgp/plugins/gr/gr_llgr.go (delete extractLocalASN; drop math import)
- internal/component/bgp/plugins/role/otc_test.go (dest.LocalAS drive; fail-closed observability test)
- internal/component/bgp/plugins/role/config_test.go (remove obsolete extractLocalASN tests)
- internal/component/bgp/plugins/gr/gr_egress_test.go (dest.LocalAS drive; TestLLGREgressIBGPClassification)
- internal/component/bgp/reactor/peer_forward_facts_test.go (assert filterInfo.LocalAS; override test)
