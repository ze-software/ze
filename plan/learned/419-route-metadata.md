# 419 -- Route Metadata

## Context

Egress filters in the forward path could only accept or reject routes, with no route context beyond raw wire bytes. Each filter needing context (OTC, LLGR stale) had to independently parse the UPDATE payload. There was also no mechanism for filters to modify routes per-peer (needed for LLGR partial deployment: NO_EXPORT + LOCAL_PREF=0 for IBGP non-LLGR peers). This spec adds generic route metadata and a per-peer modification accumulator to the forwarding pipeline.

## Decisions

- Metadata on `ReceivedUpdate`, not `PeerFilterInfo` -- metadata describes the route (set once, read many times), not the peer. Avoids per-peer copying.
- `ModAccumulator` with lazy allocation over pre-allocated map -- zero cost when no filter writes mods (the common case). Chosen over a pre-allocated map per peer which would allocate on every forward iteration.
- `IngressFilterFunc` gains shared `meta map[string]any` parameter over returning metadata separately -- filters can read what previous filters wrote, building up metadata incrementally.
- `EgressFilterFunc` gains `meta` (read) + `*ModAccumulator` (write) over returning modified payload -- keeps the filter interface simple (bool return), accumulates changes declaratively.
- Mod key convention `<action>:<target>:<name>` over flat names -- extensible, self-documenting, matches text command naming (kebab-case).
- `UpdateRouteWithMeta` SDK function over extending `UpdateRoute` signature -- backward compatible, existing callers unchanged.

## Consequences

- Any plugin can set route metadata at ingress without touching wire bytes; egress filters read it cheaply (map lookup vs byte scan).
- LLGR-4 can implement stale suppression, partial deployment, and withdrawal via ModAccumulator without changing the filter contract.
- Role OTC plugin migrated: ingress sets `meta["otc"] = true`, egress reads it (one map lookup vs N wire scans for N peers).
- `AnnounceNLRIBatch` (plugin-originated UPDATEs) does NOT go through `ForwardUpdate` -- egress filters only run on forwarded received UPDATEs. LLGR-4 must address this gap for RIB resend paths.
- Mod application (`applyMods`) is framework-only in this spec. Consuming specs register handlers for specific mod keys.

## Gotchas

- Plugin-originated UPDATEs bypass `ForwardUpdate` entirely. The `AnnounceNLRIBatch` path builds and sends directly to peers without egress filtering. RPC meta plumbing exists (`UpdateRouteInput.Meta` -> `CommandContext.Meta`) but the reactor command path doesn't create `ReceivedUpdate` from it. LLGR-4 must route RIB resends through the filter chain.
- `sed` replace on test files damaged function definitions (`func TestOTCEgressFilter(t *testing.T, nil, nil)`) -- always verify function signatures after bulk sed on test files.
- Filter panic behavior changed from fail-open to fail-closed. Ingress panic rejects the route; egress panic suppresses it for that peer. This is safer -- a buggy filter should not cause unfiltered routes to be accepted.
- OTC egress tests that pass `withOTC` payload must also pass `meta` with `"otc": true`, since the egress filter no longer parses wire bytes. Test-only meta must mirror what ingress would have set.
- Empty ingress meta map is not stored on ReceivedUpdate -- `routeMeta` stays nil unless at least one filter wrote to `ingressMeta`. Avoids storing empty maps on every cached UPDATE.

## Files

- `internal/component/plugin/registry/registry.go` -- ModAccumulator type, filter signature changes
- `internal/component/bgp/reactor/received_update.go` -- Meta field
- `internal/component/bgp/reactor/reactor_notify.go` -- safe wrappers, ingress chain meta collection
- `internal/component/bgp/reactor/reactor_api_forward.go` -- per-peer mods in ForwardUpdate
- `internal/component/bgp/plugins/role/otc.go` -- updated filter signatures
- `pkg/plugin/rpc/types.go` -- UpdateRouteInput.Meta
- `pkg/plugin/sdk/sdk_engine.go` -- UpdateRouteWithMeta
- `internal/component/plugin/server/command.go` -- CommandContext.Meta
- `internal/component/plugin/server/dispatch.go` -- RPC meta plumbing
- `docs/architecture/core-design.md` -- documented metadata and mods in forward path
- `docs/architecture/meta/README.md` -- meta key registry
- `docs/architecture/meta/role.md` -- role plugin meta key documentation
