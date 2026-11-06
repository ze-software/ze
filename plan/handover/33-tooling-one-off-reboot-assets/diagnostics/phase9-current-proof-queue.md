# Phase 9 current-source proof queue

This is continuation state from a source-only audit on 2026-11-06, not proof.
No overlay, test, build, formatter, Go tool or native scenario was executed by
this agent. All mutation candidates below need current-source anchors,
independent judgment where required, and MAIN's serialized execution in the
main checkout. A startup or build failure is not a feature-discrimination RED.

## Current status

All 15 historical scenario directories and their registered checker bodies
exist. Their registration is `specialCheckers` in
`internal/le/interoplab/bgp/check_special.go`; their bodies are in
`check_rfc.go`. The corrected spec still requires a native attributed RED and
restored GREEN per feature, with each pinned peer's renderer/schema verified.
Helper-test results do not discharge these obligations. OTC is not a member of
this incident population. Do not assert a fixed total registry count.

The durable evidence page records five RTR artifact-only CLEAN observations:
`d8ef42f8`, `af7d0304`, `18ef1991`, `28177bee`, and `2a704202`, using the
reported frozen LE40cf and DUT66cb artifacts. It separately records prepared,
unexecuted source-causal cuts. These are observations preserved in prose;
the earlier raw logs and overlay bytes are unavailable. No source-causal RED
or rebuilt restoration is established by those entries.

## Fifteen checker candidates

Checker bodies below belong to `internal/le/interoplab/bgp/check_rfc.go`.
Unless a path begins with `internal/`, producer paths are relative to
`internal/component/bgp/`. These are candidates, not approved executable patches.

| Checker / exact scenario selector | Required observable and product-producer mutation candidate |
|-----------------------------------|------------------------------------------------------------|
| `checkAddPathReadvertiseCollision` / `bgp-addpath-readvertise-collision-frr` | FRR negotiates receive ADD-PATH, holds distinct origins/IDs, and keeps IDs after an observed new session. `reactor/forward_path_id.go` `generate`/`generatePath`: separately collapse identities and remove stable map reuse. |
| `checkLocalPrefStrip` / `bgp-local-pref-strip-gobgp` | Three prefixes arrive with source controls; the complete declared destination wire interval lacks LOCAL_PREF. `reactor/forward_local_pref.go` `applyFactsLocalPref`: omit external suppression. GoBGP RIB absence alone is insufficient. |
| `checkMEDAcrossAS` / `bgp-med-across-as-gobgp` | Relayed MED is absent while Ze-originated MED 42 survives. `reactor/forward_med.go` `applyFactsMED`: omit received-MED suppression. A separate originated-MED emission corruption still needs exact writer mapping. |
| `checkMEDRemovalConfiguration` / `bgp-med-remove-configured-gobgp` | Matched route has no MED; unmatched control keeps MED 100. `policychain/external_attributes.go` `removeMED`: omit `Sets.RemoveMED`. Preserve import-side policy and delivery controls. |
| `checkRelayWithdrawalShape` / `bgp-relay-withdraw-shape-frr` | Advertisement carries prepend; withdrawal is decoded without attribute error and session survives. Separately omit ordinary protocol prepend and introduce AS_PATH into emitted withdrawal. The latter still needs exact reached writer mapping; deleting only one guard may leave a second guard effective. |
| `checkRFC2545NextHops` / `bgp-rfc2545-linklocal-nexthop-frr` | On-link global/link-local pair is used for installation; off-link route has global only. `reactor/forward_next_hop.go` `egressLinkLocal`: omit required link-local; separately bypass its scope restriction. |
| `checkRFC7606MixedUpdate` / `bgp-rfc7606-relay-shape-frr` | Installed seed is removed, announcements installed, and strict split-wire proof passes. `wireu/split.go` `buildCombinedUpdates`: separately lose the withdrawal and corrupt split announcement attributes. Exact duplicate-NEXT_HOP writer mapping remains to be completed. |
| `checkRFC7606TypedNLRIDiscard` / `bgp-rfc7606-typed-nlri-discard` | Complete valid OPEN/Established capture contains the exact assigned type-2 and no unassigned route. `plugins/nlri/evpn/rfc7606.go` `RecognizeNLRI`: admit unassigned type; separately discard assigned type. Preserve framing and completeness checks. |
| `checkRFC7999Blackhole` / `bgp-rfc7999-blackhole-frr` | Kernel discards only covered/agreed route and positively forwards both controls. `plugins/rib/rib_blackhole.go` `blackholeRouteType`: separately omit authorization and community checks. |
| `checkRouteServerASPath` / `bgp-route-server-frr` | Receiving BIRD holds exact AS_PATH `65002`; both sessions survive. Protocol prepend branches in `reactor/forward_rs.go` and `reactor/reactor_api_forward.go`: prepend toward RS client. Bind the mutation to the rail the native scenario actually reaches. |
| `checkSelfNextHopWithheld` / `bgp-self-nexthop-withheld-frr` | Same FRR incoming decode includes control and no self-next-hop subject; control has expected sole next hop. `reactor/forward_next_hop.go` `egressNextHopIsPeerOwn`: omit self-address detection. |
| `checkNoExportBoundary` / `bgp-wellknown-noexport-frr` | Internal receipt, external withholding and both control deliveries. `reactor/forward_wellknown.go` `wellKnownAllowsEgress`: omit external prohibition. A separate blanket suppression must fail internal receipt/control delivery. |
| `checkISISDynamicHostname` / `isis-p2p-frr` | FRR database names Ze's LSP; adjacency stays Up. `internal/plugins/isis/lsdb/origination.go` `fixedTLVs`: omit hostname TLV. |
| `checkNoFamilyEndOfRIB` / `no-family-peer-eor-frr` | Original-session IPv4 EOR, asymmetric OPENs and route receipt. `reactor/peer_initial_sync.go` `sendEORFamilies`: omit initial marker without skipping write-hold cleanup. Retain the separate implicit-family control in `internal/core/bgp/capability/negotiated.go` `Negotiate`. |
| `checkNSSADefault` / `ospf-stub-nssa-frr` | Type-7 default installed via border router; complete first-epoch wire has no Type-5. `internal/plugins/ospf/nssa.go` `wantsType7Default`: restore erroneous operator gate. Separately bypass NSSA exclusion in `internal/plugins/ospf/lsdb/flooding.go` `eligibleInterface`. Receiver database absence alone cannot prove no flood. |

Preserve each unchanged checker's positive delivery, session, source and capture
controls. Do not mutate checker predicates or force checker errors. Product
mutation locations above were source-read; whether each proposed cut is reached
and discriminated remains a runtime question. Preserve RFC quotes/claims/tags
and use the required native recorder/judgment routes when their units change.

## Five RTR source-bound candidates

Each exact selector below names an existing `test/plugin/<selector>.ci` file.

| Exact selector | Product-producer mutation candidate and required semantic failure |
|----------------|-----------------------------------------------------------------|
| `bgp-rr-rpki-pending-withdraw-relay` | `reactor/session_ownership.go` `adjOutWrite.allow`: change only forwarded-withdraw admission where `entry == nil`, after `dropReleasedOwner`, to allow it. Preserve later owner checks; do not create a nil panic. Observer must reject unsolicited delivery before release or the destination's exact fence-only expectation must fail. |
| `bgp-rs-rpki-pending-withdraw-relay` | Exercise that same admission defect independently through RS, retaining its own source path and exact fence bytes. |
| `bgp-rr-rpki-revalidate-relay` | `plugins/rr/validation.go` `replayValidation`: omit unicast `RelayReplayPath` delivery while retaining route validation/state. Destination conn=2 seq=2 reflected route must be missing. |
| `bgp-rs-rpki-revalidate-relay` | `plugins/rs/server_validation.go` `replayValidation`: omit its unicast delivery. Destination conn=2 seq=2 RS route must be missing. |
| `rpki-revalidate-late-sync` | `plugins/adj_rib_in/rib_validation.go` `applyToInstalled`: omit updates to `ValidationState` and `Ineligible`, preserving initial installation. Post-sync Invalid/ineligible observation must fail while original received bytes remain controlled. |

The shared harness break remains separate: re-anchor the `le mock rtr` child at
repository root. Every carrier must then fail actual synchronization, not pass
through absence. Current `Runner.childWorkingDirectory` and `leMockArea` put
the mock beside its observer. `rtrMockWaitRelease` waits for the relative
`rpki-sync.release` file before responding to reset/serial queries. The three
observer methods in `plugin_fixture_15_rpki.go` release it only after their
specific initial route/VRP/destination barriers. Listening is not synchronization.

Reconstruct the old overlays from current producers rather than assuming their
lost source anchors still match: the current ownership writer has changed
substantially. Independently review each concrete overlay, retain source and
mapping hashes, then build clean, mutation and restored binaries in main.

## MAIN-only command routes

For each interop selector, use a fresh named launcher and normal harness builds;
do not reuse a shared mutable image tag or set `NO_BUILD=1`:

```sh
env -u NO_BUILD \
  CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 \
  CGO_ENABLED=0 INTEROP_SCENARIO=<exact-scenario> \
  ZE_INTEROP_SUFFIX=<unique-owned-suffix> \
  bin/le-tooling-resume-01a1a907/le job run \
  label <unique-label> quiet command \
  ./le --name tooling-resume-01a1a907 test integration interop
```

For a single RTR carrier, use the exact positional selector, not a whole suite:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=0 \
  bin/le-tooling-resume-01a1a907/le job run \
  label <unique-label> quiet command \
  ./le --name tooling-resume-01a1a907 test bgp plugin <exact-selector>
```

For an explicitly overlay-built DUT, set both `ZE_BIN=<absolute-owned-binary>`
and `LE_TEST_NO_BUILD=1`; otherwise `Runner.Build` overwrites the selected binary.
The runner's own le supplies compiled fixture registrations and must be current.
No command above was run by this agent.

The current default FRR image declaration is `interoplab.FRRImage` in
`internal/le/interoplab/discover.go`: 10.7.1, pinned by multi-architecture digest.
Actual native proofs still require consumed image/binary identities and the
current pinned renderer/schema. Older 10.3.1 comments are not current identity
evidence. Copy completed logs, captures and manifests into the private durable
archive before volatile scratch can be removed. A fresh native capture cannot
recreate missing historical evidence.
