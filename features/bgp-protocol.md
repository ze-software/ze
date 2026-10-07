# BGP Protocol

## Meta

| Field | Value |
|-------|-------|
| Name | BGP Protocol |
| Page | docs/features/bgp-protocol.md |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 4271 iBGP MED re-advertisement (plan/immediate/spec-rfc4271-med-ibgp-readvertisement.md), RFC 4724 RFC 6793 and RFC 8654 obligations the ledger marks Partial, RFC 7606 gap row in rfc/short/rfc7606.md |
| Level | experimental |
| Components | internal/component/bgp, cmd/ze, internal/component/config/infra |
| Real-path tests | test/plugin/bgp-update-delay.ci, test/plugin/bgp-update-delay-converges.ci, test/plugin/bgp-update-delay-validation.ci, test/ui/bgp-update-delay-command.ci |
| Interop | bgp/bgp-update-delay-frr, bgp/bgp-ebgp-ipv4-frr, bgp/bgp-ibgp-frr, bgp/bgp-ebgp-gobgp, bgp/bgp-routes-from-bird |
| RFCs | rfc4271, rfc4760, rfc6793, rfc4724, rfc7606, rfc8654 |
| Docs | docs/features/bgp-protocol.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; update-delay holds until End-of-RIB/establish-wait/max-delay per the bgp-update-delay .ci headers; the 23/14/15 counts were not re-derived |
| Defect review | 2026-10-07: plan/immediate/spec-rfc4271-med-ibgp-readvertisement.md open; plan/pre-release/spec-rfc-verdict-fix-bgp.md open; journal helper-bypassed-by-an-open-coded-copy forward-rail rows taken as open |

## Description

23 address families, 14 capabilities, 15 path attributes. `bgp update-delay max-delay <seconds>` holds the first advertisement of a speaker that has just started until every configured peer has sent its End-of-RIB marker, until an optional `establish-wait` expires, or until `max-delay` expires, so a neighbor receives one settled route set instead of an initial set followed by corrections; RFC 4724 Section 4.1 prescribes the same deferral for a restarting speaker and requires the configurable upper bound. `show bgp update-delay` reports whether the speaker is holding and what it waits for. <!-- source: internal/component/bgp/reactor/update_delay.go -- updateDelayHold --> The whole BGP subsystem is compile-out-able with the `ze_bgp` build tag. The default feature list includes it, while bare `ze_core` builds drop `internal/component/bgp` entirely -- engine, wire codec, RIB, and every BGP plugin -- plus `flowspec-firewall`, which translates BGP-delivered flowspec routes and is meaningless without it. Both composition roots are gated (the generated `all_ze_bgp.go`, and the `package main` root, which splits into `cmd/ze/dispatch_bgp.go` for the CLI registration under `ze_core && ze_bgp` and `cmd/ze/infra_bgp.go` for the always-on seams under `ze_bgp` alone), and a `bgp {}` config block is rejected as an unknown field rather than silently ignored. A BGP-less daemon still runs OSPF, IS-IS, static routes, the FIB, MRT recording, and flow export: the route-action vocabulary, the BGP message-type codes, and the best-path-change event contract those consumers share live in always-on `internal/core/bgp/*` leaves, and the places always-on code used to call into the engine are inversion-of-control seams the engine fills from its own `init()`: `internal/component/config/infra` carries four of them (`ze config dump/diff/validate` resolution, BGP peer validation, roleless-peer reporting, and the graceful-restart marker), and the MRT RIB dump provider is a fifth. `le chaos run` and `le perf` drive an in-process reactor and force `ze_bgp` on. Within a `ze_bgp` build the BMP monitoring plugin (RFC 7854/8671/9069) is independently compile-out-able with the `ze_bmp` build tag: it is a dependent gate. BMP imports the engine, so `all_ze_bmp.go` is generated with `//go:build ze_bgp && ze_bmp`, and a `ze_bmp`-without-`ze_bgp` build links neither BMP nor the engine. <!-- source: feature-gates.txt -- ze_bgp --> <!-- source: internal/le/repo/featuretags/daemontags.go -- DaemonTags --> <!-- source: internal/component/plugin/all/all_ze_bgp.go -- gated BGP imports --> <!-- source: cmd/ze/dispatch_bgp.go -- gated BGP CLI dispatch root --> <!-- source: cmd/ze/infra_bgp.go -- gated BGP infra-seam link --> <!-- source: internal/component/config/infra/bgp.go -- always-on BGP seams --> <!-- source: feature-gates.txt -- ze_bmp --> <!-- source: internal/component/plugin/all/all_ze_bmp.go -- gated BMP imports -->
