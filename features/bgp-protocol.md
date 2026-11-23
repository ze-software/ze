# BGP Protocol

## Meta

| Field | Value |
|-------|-------|
| Name | BGP Protocol |
| Page | docs/features/bgp-protocol.md |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 4271 iBGP MED re-advertisement (gap row RFC4271-9.1.2.2-2 and plan/release/fixes/spec-rfc4271-med-ibgp-readvertisement.md), RFC 4271 MinRouteAdvertisementIntervalTimer (gap rows RFC4271-9.2.1.1-2 and RFC4271-9.2.1.1-3), RFC 4271 rows RFC4271-5.1.6-1 RFC4271-6.2-3 RFC4271-9.2-2 and RFC4271-9.2-3 the ledger marks partial, RFC 8654 row RFC8654-5-4 the ledger marks partial |
| Level | experimental |
| Components | internal/component/bgp, cmd/ze, internal/component/config/infra |
| Real-path tests | test/plugin/bgp-update-delay.ci, test/plugin/bgp-update-delay-converges.ci, test/plugin/bgp-update-delay-validation.ci, test/ui/bgp-update-delay-command.ci, test/plugin/rfc8212-ebgp-no-import-policy.ci, test/plugin/rfc8212-ebgp-no-export-policy.ci, test/plugin/rfc8212-ibgp-no-policy-unchanged.ci, test/reload/rfc8212-explicit-policy-reload.ci, test/plugin/bgp-delay-open.ci, test/plugin/bgp-delay-open-bfd-strict.ci, test/plugin/role-incoming-open-isolation.ci, test/plugin/role-incoming-open-isolation-forked.ci, test/plugin/incoming-preopen-update-isolation.ci, test/plugin/incoming-open-learned-isolation.ci, test/plugin/incoming-open-learned-isolation-forked.ci, test/plugin/bgp-best-route-transit-default.ci, test/plugin/bgp-best-route-needs-export-policy.ci, test/plugin/bgp-best-route-refresh-selected.ci |
| Interop | bgp/bgp-update-delay-frr, bgp/bgp-ebgp-ipv4-frr, bgp/bgp-ibgp-frr, bgp/bgp-ebgp-gobgp, bgp/bgp-routes-from-bird, bgp/bgp-rfc8212-default-reject-frr, bgp/bgp-delayopen-frr, bgp/bgp-best-route-transit-frr, bgp/bgp-best-route-transit-bird |
| RFCs | rfc4271, rfc4760, rfc6793, rfc4724, rfc7606, rfc8654, rfc8212 |
| Docs | docs/features/bgp-protocol.md |
| Doc review | 2026-11-23: the selection deferral paragraph and the bgp aggregate section added to docs/features/bgp-protocol.md carry the facts and anchors of this declaration's two paragraphs and contradict nothing in it; the aggregation paragraph matches internal/component/bgp/plugins/rib/aggregate.go (parseAggregateRule with aggregator default true, synthesizeAggregate refusing differing MED, aggregateOrigin, aggregateNextHop accepting only a local address, aggregateBriefASPath with no AS_SET, aggregateAtomic, aggregateOwnAggregator, recomputeAggregateRule logging the suppression, publishAggregateRoute) and list aggregate in ze-bgp-conf.yang, with only unit tests (rfc4271_aggregate_test.go) and no .ci or interop scenario configuring it; the selection deferral sentence matches selectionDeferral, onPeerNegotiated, evaluateReleasesLocked and parseSelectionDeferralConfig in selection_deferral.go; the Scope gaps match the unchecked partial and gap rows of rfc/short/rfc4271.md and rfc/short/rfc8654.md, rfc4724.md and rfc6793.md and rfc7606.md holding none. Earlier, 2026-11-19: checked the ordinary transit grants, export policy, and refresh against the current source and the November 14 third-party next-hop change against the neighbor table lookup and watch. The separate LLGR readvertisement paragraph omits the policy-change branch in `runStaleChain`. |
| Defect review | 2026-10-07: plan/release/fixes/spec-rfc4271-med-ibgp-readvertisement.md open; spec-rfc-verdict-fix-bgp open; journal helper-bypassed-by-an-open-coded-copy forward-rail rows taken as open |

## Description

For ordinary BGP transit, attach the same `bgp-rib` process to the source and
destination peers. Ze advertises the selected learned route to each destination
whose export policy accepts it. On a new session or a route refresh, Ze sends
the current exportable selection. [Ordinary transit configuration](../docs/guide/route-reflection.md#without-route-reflection)
<!-- source: internal/component/plugin/server/dissemination_validate.go -- ValidateDisseminationBindings -->
<!-- source: internal/component/bgp/plugins/rib/rib_ordinary_destination.go -- updateSelectedDestination -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_refresh.go -- requestRefresh -->
<!-- source: internal/component/bgp/filterapi/export_kind.go -- UnconfiguredExportAccepts -->

`bgp peer <address> timer delay-open <seconds>` holds the local OPEN until the
remote OPEN arrives or the configured 1–240 second timer expires. Zero disables
the wait. Strict BFD keeps the handshake bounded while waiting for BFD Up.
An incoming connection validates its role without changing an incumbent's
learned relationship; selection publishes its retained OPEN pair before peer-up.
<!-- source: internal/component/bgp/reactor/session_connection.go -- connectionEstablished -->
<!-- source: internal/component/bgp/reactor/session_bfd_strict.go -- advanceAfterOpen -->
<!-- source: internal/component/bgp/reactor/session_open_admission.go -- selectNegotiationEpoch -->

`bgp aggregate <prefix>` makes `bgp-rib` synthesize one IPv4 or IPv6
aggregate route while selected routes more specific than the prefix exist,
following RFC 4271 Section 9.2.2.2, install it in the Loc-RIB and announce it.
Routes whose MULTI_EXIT_DISC differs are not aggregated: the aggregate is
suppressed and a warning names the prefix. ORIGIN is the worst of the
contributors (INCOMPLETE, then EGP, then IGP). When the contributors'
NEXT_HOPs differ, `next-hop` names the local address to use, and the
aggregate is suppressed when that address is not local; without it Ze picks a local session or interface address of
the family. The AS_PATH is the contributors' common leading AS_SEQUENCE and
never holds an AS_SET, and ATOMIC_AGGREGATE is set when a contributor carries
it or when AS numbers were left out. The contributors' AGGREGATOR attributes
are discarded, and Ze attaches its own (local AS and router ID) unless
`aggregator false`. `brief-origin-as <asn>` truncates the AS_PATH after the
right-most instance of that AS, the consistent brief aggregation of RFC 9774
Section 5.2. Unit tests cover the synthesis; no functional test or interop
scenario configures aggregation yet.
<!-- source: internal/component/bgp/plugins/rib/aggregate.go -- parseAggregateRule, synthesizeAggregate, aggregateCheckMED, aggregateOrigin, aggregateNextHop, aggregateBriefASPath, aggregateAtomic, aggregateOwnAggregator, recomputeAggregateRule, publishAggregateRoute -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- list aggregate -->

23 address families, 14 capabilities, 15 path attributes. `bgp update-delay max-delay <seconds>` holds the first advertisement of a speaker that has just started until every configured peer has sent its End-of-RIB marker, until an optional `establish-wait` expires, or until `max-delay` expires, so a neighbor receives one settled route set instead of an initial set followed by corrections. `show bgp update-delay` reports whether the speaker is holding and what it waits for. <!-- source: internal/component/bgp/reactor/update_delay.go -- updateDelayHold --> With `max-delay` set, `bgp-rib` also defers best-path selection per address family, as RFC 4724 Section 4.1 requires of a restarting speaker: a family is selected once every configured peer has negotiated and each one that advertised Graceful Restart without the Restart State bit has sent its End-of-RIB for that family, or once `max-delay` expires. A peer without the capability, or with the Restart State bit set, is not waited for. <!-- source: internal/component/bgp/plugins/rib/selection_deferral.go -- selectionDeferral, onPeerNegotiated, evaluateReleasesLocked, parseSelectionDeferralConfig --> The whole BGP subsystem is compile-out-able with the `ze_bgp` build tag. The default feature list includes it, while bare `ze_core` builds drop `internal/component/bgp` entirely -- engine, wire codec, RIB, and every BGP plugin -- plus `flowspec-firewall`, which translates BGP-delivered flowspec routes and is meaningless without it. Both composition roots are gated (the generated `all_ze_bgp.go`, and the `package main` root, which splits into `cmd/ze/dispatch_bgp.go` for the CLI registration under `ze_core && ze_bgp` and `cmd/ze/infra_bgp.go` for the always-on seams under `ze_bgp` alone), and a `bgp {}` config block is rejected as an unknown field rather than silently ignored. A BGP-less daemon still runs OSPF, IS-IS, static routes, the FIB, MRT recording, and flow export: the route-action vocabulary, the BGP message-type codes, and the best-path-change event contract those consumers share live in always-on `internal/core/bgp/*` leaves, and the places always-on code used to call into the engine are inversion-of-control seams the engine fills from its own `init()`: `internal/component/config/infra` carries four of them (`ze config dump/diff/validate` resolution, BGP peer validation, roleless-peer reporting, and the graceful-restart marker), and the MRT RIB dump provider is a fifth. `le chaos run` and `le perf` drive an in-process reactor and force `ze_bgp` on. Within a `ze_bgp` build the BMP monitoring plugin (RFC 7854/8671/9069) is independently compile-out-able with the `ze_bmp` build tag: it is a dependent gate. BMP imports the engine, so `all_ze_bmp.go` is generated with `//go:build ze_bgp && ze_bmp`, and a `ze_bmp`-without-`ze_bgp` build links neither BMP nor the engine. <!-- source: feature-gates.txt -- ze_bgp --> <!-- source: internal/le/repo/featuretags/daemontags.go -- DaemonTags --> <!-- source: internal/component/plugin/all/all_ze_bgp.go -- gated BGP imports --> <!-- source: cmd/ze/dispatch_bgp.go -- gated BGP CLI dispatch root --> <!-- source: cmd/ze/infra_bgp.go -- gated BGP infra-seam link --> <!-- source: internal/component/config/infra/bgp.go -- always-on BGP seams --> <!-- source: feature-gates.txt -- ze_bmp --> <!-- source: internal/component/plugin/all/all_ze_bmp.go -- gated BMP imports -->
