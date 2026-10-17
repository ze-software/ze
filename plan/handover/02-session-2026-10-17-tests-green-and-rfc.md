# Handover: session 1b0ec3e6 (2026-10-15..17) — all tests green, then RFC MUSTs

Owner goals, in order: (1) every test passes, local full verify AND GitHub CI;
(2) then move RFC MUST requirements forward. Commit namespace `b2ec5abb`.
Rules this session added: `.claude/rules/known-reds-first.md` (targeted reruns,
full verify last on a quiet machine) and `.claude/rules/handover-in-repo.md`
(this file is the handover; machine-local notes are not). Up to 5 dev agents.
Agents test through a private launcher `./le --name <x> ...` (the shared
`bin/le` is often older than the tree) and delete `bin/le-<x>` after.

## 1. Commit pending (do first)
Uncommitted and reviewed four times: the filter modify now applies per NLRI
section, and bgp-filter-irr re-admits held routes by ROUTE-REFRESH.
- Filter text format (owner decision): `nlri add|del <prefixes>` = legacy NLRI
  and Withdrawn Routes; `mp <family> add|del <prefixes>` = MP_REACH/MP_UNREACH,
  any family. No shim. Reactor: `filter_nlri_delta.go` (subset per section,
  path-ids and labels kept, refuses additions, reorder, duplicates and
  unrewritable families), `AttrModReplaceNLRI`, and the subject rendered from
  route keys (ADD-PATH and labeled NLRI no longer reach a filter as junk).
  Interop scenario `bgp-policy-modify-mp-frr` (raw injector to FRR) is green,
  and red with the fix stubbed.
- Commit subject: `fix(bgp): filter modify applies per NLRI section; IRR re-admits routes`.
  The file set is every uncommitted path under `internal/component/bgp/`, the
  fixtures `plugin_fixture_05.go` and `plugin_fixture_07*.go`, `internal/test/mock/irr`,
  the `.ci` files `filter-irr-readmit`, `prefix-filter-modify-mp`,
  `prefix-filter-modify-partial` and `dynamic-peer-applies-group-filters`, the
  scenario directory `test/interop/scenarios/bgp-policy-modify-mp-frr/`,
  `internal/le/interoplab/bgp/register_policy_modify_mp.go`, the touched docs and
  `plan/journal` rows, and `test/weakened/b2ec5abb.md`.
- **Blocked on the owner:** RFC-tagged `filter_prefix.TestPrefixPolicyFiltersFlowSpecDestination`
  (`rfc8955_flowspec_test.go`): its only change is the subject prefix `"nlri "` → `"mp "`.
  Needs Thomas's explicit words, then `./le rfc approve unit filter_prefix.TestPrefixPolicyFiltersFlowSpecDestination reason "<his words>"`.
- **In flight in the same files:** IRR items #6 (rewritable widens to MP CIDR
  blocks, which ends a once-a-minute ROUTE-REFRESH loop) and #7 (echo
  marker-only blocks). Re-run filter_irr tests and the irr and filter `.ci`
  patterns before committing.
- After the commit: add six reviewed-claim rows to `plan/doc-reviewed/b2ec5abb.md`
  naming the new SHA, one symbol each: `runIngressPolicyChain` and
  `runEgressPolicyChainASN4` (api/commands.md), `buildModifiedPayloadWithLimit`
  (edge-cases/extended-message.md), `parseFilterAttrsInto` (perf-round-3.md),
  `mpReachNextHopHandler` (update-building.md), `runEgressPolicyChainASN4`
  (wire/attributes.md). Then `./le doc wiring check` and commit.

## 2. Waiting on the owner
- GTSM RFC 5082-tagged tests (nightly integration): the fault is the TEST.
  Commit 7278126c3f moved nft rule counters after the matches, and the test
  helpers still assume a counter at the front of each rule. Proposed: helpers
  `awaitTermMatched`, `termsMatched` and `assertNoTermMatched`; the drop test
  asserts exactly the quoted-destination term matched; one assertion is lost
  (proof the chain evaluated a non-matching packet; the ICMP receive counter
  and the installed-table check remain). Proven green, and red under a mutant
  of the drop terms. Needs owner approval, then re-record RFC5082-3-2 and -3-4.
- Startup stall under disk load: about 36 fsyncs before the first listener.
  Options A (record the boot config after the listeners), B (functional DUT
  store in memory), C both (recommended).
- Fastpath enable ignores best routes already installed (journal row): fix or not.
- Route-injection grammar (`update text ... nlri <family> add`): adopt the
  `nlri` / `mp <family>` keywords too, or keep it.
- IRR cache: keep its disk copy, or memory only.
- Delete the temporary untracked tests `zz_state_timing_diag_test.go`
  (plugin/server) and `zz_c3_profile_test.go` (le/rfc).
- Commit 7f59cfe2ad (L2TP) message lost one example phrase to shell quoting:
  amend only if Thomas asks.

## 3. Agents running at the time of writing
PPPoE `pppoe-padr-replay` (a replayed PADR loses the existing session), L2TP
interop `02-ppp-bgp-redistribute-frr` (the injected subscriber route never
reaches FRR), and the IRR #6/#7 work above. Each appends its handoff here.

## 4. Remaining known reds (not started)
- Nightly: ipsec readiness timeouts (5 scenarios); BGP interop 90-minute ssh
  deadline (suites now print per-scenario progress, so read the next CI run);
  PPPoE IPv6CP gap waits on `plan/spec-l2tp-ipv6-subscriber.md`.
- CI: the next push proves userns, the l2tp modprobe step, the race split in 3,
  golangci-lint from vendor and the gh-pages seed.
- Then a full `./le verify worktree` on a quiet machine, and ask Thomas to push.

## 5. Spec claimed
`plan/pre-release/spec-plugin-ownership-imports.md`: D1 and D2 landed (69b67933a2,
82591173b6), arch tier green. Remaining: route J (AC-17 health RPCs) and closure.

## Agent handoffs (appended below)
