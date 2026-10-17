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
- IRR items #6 (rewritable widens to MP CIDR blocks incl. mpls-label, ending
  the refresh loop for them) and #7 (marker blocks echoed) are done and
  reviewed; the final review items are applied (compound guards split,
  filter_prefix reads filtertext.CIDRFamily, marker-echo rule in
  process-protocol.md, filter-irr-modify-mp.ci proven red/green). New file
  set additions: `internal/component/bgp/filtertext/cidr*.go`,
  `filter_irr/testmain_test.go`, `test/plugin/filter-irr-modify-mp.ci`, two
  journal rows. Gates green: race tests, irr 6/6, filter 35/35, scoped lint,
  compound-guard. Only `./le doc wiring check` stays red on the five claims
  below, which need the post-commit rows.
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

## 3. Done since this handover was first written
PPPoE `pppoe-padr-replay` fixed in the checker (8dce48dd9d); L2TP interop
`02-ppp-bgp-redistribute-frr` fixed in its frr.conf (b3a5960dc7). No agent
is running.

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

### 2026-10-17 injection grammar `nlri add|del` / `mp <family> add|del` (agent ran out of budget, NOTHING COMMITTED)

Owner decision: `update text` and `update cursor` take the filter-text keywords. Done in the
working tree (uncommitted, all mine unless named):

- Parser: `internal/component/bgp/plugins/cmd/update/update_text_nlri.go` (parseNLRISection
  dispatches `nlri` to parseLegacySection, `mp` to parseMPSection; parseINETSection shared),
  `update_text.go` (kwMP, boundary keyword, grammar comment), `cursor.go`. Old `nlri <family>`
  refused by name (removedFamilyFormError). `nlri add` with an explicit IPv6 next hop refused,
  naming `mp ipv4/unicast add`. `mp ipv4/unicast` accepted ONLY for add with an explicit IPv6
  next hop (RFC 8950); its del, eor, IPv4 or self next hop refused, naming the `nlri` form,
  because the reactor (inlineIPv4Unicast) cannot put IPv4-next-hop IPv4 unicast in MP_REACH.
  OWNER QUESTION.
- New tests: `update_text_section_keyword_test.go`. Package green.
- Helper: `internal/component/bgp/textparse/section.go` (KWNLRISection, KWMPSection,
  LegacyNLRISection, IPv6NextHopText).
- Producers switched: bgp/format.go (announce and withdraw text), rib/rib_replay.go (cursor),
  redistribute/consumer.go, ddos/flowspec/responder.go, reactor/peer_initial_sync.go (default
  originate FILTER text, which 6019dd0675 missed), rr/rr.go, rs/server_handlers.go (withdraw and
  EOR), exabgp bridge (bridge_command.go, bridge_flow.go, bridge_route_forms.go EOR,
  bridge_attribute.go NextHopIPv6), fixtures 06_decoder, 09, 02_events, cmd/ze/help_ai.go.
- Rewritten by script (lines holding `update text` or `update cursor`): about 880 lines in about
  150 files under test/ and internal/ (.ci and Go).

Left to do:
1. rr and rs tests: TestPeerUpReflectsExistingFlowSpecBeforeEOR (rr validation_test.go),
   TestReplayForPeer_SendsEOR, TestPeerUpReplaysExistingFlowSpecBeforeEOR (rs
   server_validation_test.go): expectations still match the old `nlri <fam> eor` text.
   rs/server_retention_test.go is ANOTHER session's file, not touched.
2. The rib package does not build because of another session's forward_tracker_test.go; rerun
   rib tests once it builds (my rib_test, rib_replay_test, llgr_sent_lifecycle_test edits).
3. Docs not yet updated: docs/guide/route-injection.md, docs/architecture/api/commands.md
   (update text section), update-syntax.md, architecture.md, text-format.md, plugin-modes.md,
   ipc_protocol.md, json-format.md, DESIGN.md, core-design.md, docs/exabgp/exabgp-migration.md.
   Only command lines change; event text (`received update ... nlri <fam> add`) does not.
4. Functional proof: a .ci that sends `nlri ipv4/unicast add` through the real entry point and
   asserts the error text; run targeted .ci tests for injection (test/encode, test/plugin).
5. Independent review (ze-read agent), then commit (main thread; this agent holds no Agent).
6. Out of scope, owner to decide: `update hex` and `update b64` still use `nlri <family>`;
   config `update { nlri { <family> add ... } }` and the received-event text format are
   unchanged; `nlri add next-hop self` may still be moved into MP_REACH by the reactor when
   self resolves to IPv6.
7. plugin/server and reactor RFC8654 package failures seen in one run look unrelated
   (namespace `bgp` not registered); not investigated.
