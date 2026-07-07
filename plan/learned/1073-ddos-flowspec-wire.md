# 1073 -- DDoS FlowSpec Origination (wire the responder + announce flowspec verb)

## Context

The `ddos-flowspec` responder (cp-survival-5) shipped with `announceFunc`/`withdrawFunc` as logging-only stubs ("cp-survival-4 not yet wired"), and cp-survival-4 left the `announce ... flowspec` CLI branch (`handleAnnounceFlowspec`) as a "not yet implemented" stub. A handover claimed FlowSpec origination "already works end-to-end" and only the responder needed a dispatcher. This spec wired both stubs so a characterized DDoS attack originates a real upstream FlowSpec rule, and operators can originate one on demand. It also corrected the config model per the user: `action` mandatory/no-default (rate-limit|discard), `rate-limit-bytes` range 0..max required only for rate-limit, `rate-limit 0 == discard`, no fabricated rate.

## Decisions

- Two separate code paths, not one shared path: the responder (separate process) originates via `p.UpdateRoute` update-text; the CLI verb (in-process BGP plugin) builds an `NLRIBatch` and calls `announceAndTrack`. They share only RFC 8955 semantics -- the process boundary forbids the responder from using the in-process tag registry (there is NO `tag.*` meta consumer; the registry is driven only by the `ze-bgp:announce`/`withdraw` RPCs). This corrects the handover's "shared tracked path" framing.
- Responder seam is a `routeDispatcher` interface field injected via `newResponder(cfg, dispatcher)` over reassigning package-var stubs; one shared `renderFlowspecCommand(match, action, rate, mode)` for announce (`add`) and withdraw (`del`) so the withdrawn NLRI byte-matches the announced one.
- CLI verb builds the FlowSpec NLRI via `registry.EncodeNLRIByFamily` (core infra) + `nlri.NewWireNLRI`, reusing the flowspec parser through the registration seam -- no plugin->plugin import. Action ext-community via `route.ParseExtendedCommunities` (`discard` / `traffic-rate 0 <bps> bytes`).
- Selector `"*"` (engine self-scopes flow NLRI to peers that negotiated the family). v4+v6 via `DstPrefix.Addr().Is6()`.

## Consequences

- DDoS auto-mitigation now actually reaches the wire (it never did before -- see Gotchas). `docs/guide/ddos-mitigation.md` updated: `action` mandatory, `rate-limit-bytes` documented, example fixed.
- The `announce flowspec <components> (rate-limit <bps>|discard) [tag ..][for ..]` CLI verb works and is tracked/withdrawable via the existing registry.
- Config: `Config.rateLimitBytesSet` distinguishes absent (error for rate-limit) from explicit 0 (valid == discard). `DefaultConfig()` no longer presets an action.

## Gotchas

- **The responder never reached the wire, and no unit/integration test caught it.** A FlowSpec origination with NO next-hop is silently dropped before the wire -- the MP_REACH_NLRI requires one (interop scenarios 24/30 comment "nhop required for MP_REACH_NLRI"). `renderFlowspecCommand` emitted none. Unit tests + a `ParseUpdateText` integration test all passed because they validate NLRI *parsing*; the drop is in the *send* path. Only the peer-based `test/plugin/ddos-flowspec-announce.ci` (peer receives the UPDATE) caught it. Fix: emit `nhop self` on `add` (withdraw/MP_UNREACH needs none). Lesson: a peer-based functional test is not optional for origination features -- parsing green != sending works.
- The update-text flow encoder (`plugin_encode_text.go` `parseProtocolComponentText`) rejected `protocol =6` ("invalid protocol") -- it accepted only bare numbers/names while ports accept `=`. Fixed to strip a leading `=` so `=17` and `17` parse identically (user directive).
- There is NO `ln` keyword in Ze grammar (a garbled `rg` output suggested one). The traffic-action clause is `extended-community [rate-limit:<bps>]`; `rate-limit:0 == discard` on the wire.
- The `announce flowspec` tracked verb cannot be driven peer-based in the plugin `.ci` harness: `cmd=api` reaches the route/update API, not the CLI-command dispatch; the harness has no CLI/SSH directive. Covered instead by `TestHandleAnnounceFlowspec` (fake reactor embedding `bgptypes.BGPReactor`, overriding only `AnnounceNLRIBatch`) -- matching cp-survival-4's Go-handler-test approach.

## Files

- `internal/plugins/ddos/flowspec/`: `responder.go` (dispatcher field + `renderFlowspecCommand` with `nhop self`), `register.go` (`sdkDispatcher`), `config.go` (presence-aware validation, action constants), `yang/ze-ddos-flowspec-conf.yang` (action mandatory, rate-limit-bytes 0..max), `responder_test.go`, `config_test.go`, `show_test.go`
- `internal/component/bgp/plugins/cmd/announce/announce.go` (`handleAnnounceFlowspec` + helpers), `announce_test.go` (fake reactor handler test)
- `internal/component/bgp/plugins/nlri/flowspec/plugin_encode_text.go` (protocol `=` operator), `plugin_test.go`
- `internal/component/bgp/plugins/cmd/update/update_text_test.go` (responder-grammar integration test)
- `docs/guide/ddos-mitigation.md`
- `test/plugin/ddos-flowspec-announce.ci` (peer-based functional test)
