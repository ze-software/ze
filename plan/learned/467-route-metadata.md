# 467 -- Route Metadata

## Context

Egress filters in the forward path could only accept or reject routes, with no route context beyond raw wire bytes. Each filter needing context (OTC, LLGR stale) had to independently parse the UPDATE payload per destination peer. There was no mechanism for filters to modify routes per-peer (needed for LLGR partial deployment). OTC filtering was wire-byte-based rather than config-based, meaning transitive attributes lost during RIB replay would break filtering. This work adds generic route metadata, a per-peer modification accumulator, and plumbs metadata through the entire forwarding pipeline from ingress to RIB storage.

## Decisions

- Metadata on `ReceivedUpdate`, not `PeerFilterInfo` -- metadata describes the route (set once, read many times), not the peer.
- `ModAccumulator` with lazy allocation over pre-allocated map -- zero cost when no filter writes mods.
- `IngressFilterFunc` gains shared `meta map[string]any` parameter -- filters build metadata incrementally.
- `EgressFilterFunc` gains `meta` (read) + `*ModAccumulator` (write) -- keeps bool return, accumulates changes declaratively.
- Mod key convention `<action>:<target>:<name>` (set/del/add/withdraw : attr/nlri : name) over flat names.
- OTC egress uses `meta["src-role"]` from our config over wire-byte OTC detection -- config-based is correct because we know the peer relationship regardless of what the remote sends.
- `Route.RawAttrs` stores hex wire bytes over per-field replay -- preserves ALL transitive attributes (OTC, unknown) through RIB store-and-replay.
- `Route.Meta` stores route metadata on ribOut over deriving at replay time -- metadata travels with the route.
- Full meta plumbing through forward path (fwdItem -> session.sentMeta -> RawMessage.Meta -> JSON route-meta -> Event.RouteMeta -> Route.Meta) over leaving it as a gap -- enables RIB to store metadata set at ingress.
- Fail-closed panic recovery over fail-open -- a buggy filter should reject, not accept.
- `session.sentMeta` cleared via defer over manual clear after loop -- prevents stale meta on error paths.
- Hex validation in `formatAnnounceHex` over trusting the input -- defense-in-depth.

## Consequences

- Any plugin can set route metadata at ingress; egress filters read it cheaply (map lookup vs wire scan per peer).
- OTC suppression now based on our configured peer role, not wire-byte OTC presence. Routes from unconfigured peers are not filtered (intentional: no config = no opinion).
- `Route.RawAttrs` + `Route.Meta` survive RIB replay. When replay path goes through ForwardUpdate (LLGR-4), egress filters will have full metadata.
- `writeRawUpdateBody` now fires sent events (previously silent). RIB ribOut now tracks zero-copy forwarded routes, not just re-encoded ones.
- LLGR-4 can implement stale suppression, partial deployment, and withdrawal via ModAccumulator without changing the filter contract.
- `applyMods` is framework-only. Consuming specs register handlers for specific mod keys.
- `AnnounceNLRIBatch` (plugin-originated UPDATEs) still bypasses ForwardUpdate. RIB replay needs routing through ForwardUpdate for egress filters to apply.

## Gotchas

- `session.sentMeta` must be cleared via defer, not after the loop -- early returns on write errors would leak stale meta to subsequent writes on the same session.
- `writeRawUpdateBody` firing sent events extends `writeMu` hold time per raw body write (N event deliveries per batch). Monitor for latency impact under high forwarding load.
- JSON round-trip converts numeric meta values to float64. Current usage (string values) unaffected. Future numeric meta must use comma-ok type assertions.
- `Route.Meta` is a map (reference type). Struct copies of Route share the same Meta map. Currently safe (Meta is read-only after creation), but deep copy needed if routes become mutable.
- OTC egress no longer checks wire bytes for unconfigured peers. RFC 9234 Section 5 says routes with OTC "MUST NOT" be sent to Provider/Peer/RS -- the new logic only enforces when source peer has a configured role. This is the user's explicit design choice.

## Files

- `internal/component/plugin/registry/registry.go` -- ModAccumulator type, filter signature changes
- `internal/component/bgp/reactor/received_update.go` -- Meta field
- `internal/component/bgp/reactor/reactor_notify.go` -- safe wrappers (fail-closed), ingress chain meta
- `internal/component/bgp/reactor/reactor_api_forward.go` -- per-peer mods, fwdItem.meta
- `internal/component/bgp/reactor/forward_pool.go` -- session.sentMeta lifecycle (defer clear)
- `internal/component/bgp/reactor/session.go` -- sentMeta field, MessageCallback meta param
- `internal/component/bgp/reactor/session_write.go` -- writeRawUpdateBody sent events, sentMeta plumbing
- `internal/component/bgp/reactor/session_read.go` -- MessageCallback meta param (nil for received)
- `internal/component/bgp/types/rawmessage.go` -- RawMessage.Meta field
- `internal/component/bgp/event.go` -- Event.RouteMeta field
- `internal/component/bgp/route.go` -- Route.RawAttrs, Route.Meta fields
- `internal/component/bgp/format.go` -- formatAnnounceHex with hex validation, text fallback
- `internal/component/bgp/format/text.go` -- route-meta JSON injection in formatFullFromResult
- `internal/component/bgp/plugins/role/otc.go` -- meta["src-role"] ingress/egress, wire fallback removed
- `internal/component/bgp/plugins/rib/rib.go` -- handleSent stores RawAttrs + RouteMeta on Route
- `internal/component/bgp/plugins/watchdog/config.go` -- Route passed by pointer (hugeParam fix)
- `pkg/plugin/rpc/types.go` -- UpdateRouteInput.Meta field
- `pkg/plugin/sdk/sdk_engine.go` -- UpdateRouteWithMeta function
- `internal/component/plugin/server/command.go` -- CommandContext.Meta field
- `internal/component/plugin/server/dispatch.go` -- RPC + DirectBridge meta plumbing
- `docs/architecture/core-design.md` -- metadata + mods in forward path diagram
- `docs/architecture/meta/README.md` -- meta key registry
- `docs/architecture/meta/role.md` -- role plugin meta key documentation
