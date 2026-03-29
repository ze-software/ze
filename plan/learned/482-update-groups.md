# 482 -- Update Groups

## Context

When Ze sends route updates to N peers with identical capabilities, it builds the same UPDATE message N times independently. For a route server with 100 peers, this means 100 identical builds. Update groups eliminate this redundancy: peers with the same encoding context (ContextID) share a single UPDATE build, reducing work to 1 build + N sends.

The user also required ExaBGP compatibility: migrated configs must preserve per-peer UPDATE building (matching ExaBGP behavior) via an env var that defaults to enabled for native Ze configs.

## Decisions

- Chose `ze.bgp.reactor.update-groups` as the env var (reactor section) over a per-peer config knob, because update groups are a reactor-level optimization that applies globally.
- Chose GroupKey = `{ContextID, PolicyKey}` over ContextID alone, to allow future per-peer outbound policy differentiation (PolicyKey is 0 for all peers today).
- Chose mutex protection over single-threaded assumption, because Add/Remove are called from peer goroutines (via FSM lifecycle events), not the reactor event loop. Discovered via chaos test panic.
- Chose stored GroupKey on Peer struct over reading sendCtxID at Remove time, because clearEncodingContexts zeros sendCtxID before Remove can read it in some code paths.
- Chose ExaBGP migration injection (automatic) over requiring users to manually configure the setting, because users migrating from ExaBGP expect identical behavior by default.
- GroupsForPeers builds a fresh temporary grouping from the peer subset (not from the persistent index), because callers pass filtered subsets and the index may contain peers not in the current operation.

## Consequences

- Route server and route reflector deployments see proportional speedup (N peers / K groups fewer UPDATE builds).
- ExaBGP migrated configs work identically to ExaBGP until user explicitly removes `update-groups false`.
- Future per-peer outbound policy can use PolicyKey to further subdivide groups without changing the data structure.
- The mutex adds negligible overhead (uncontended in normal operation; only contended during rapid peer churn in chaos tests).

## Gotchas

- Peer goroutines call notifyPeerEstablished/notifyPeerClosed, which call Add/Remove on the index. The original "single-threaded reactor" assumption was wrong for these lifecycle callbacks. The chaos test caught this as a bounds panic.
- sendCtxID can be zeroed by clearEncodingContexts before Remove reads it. Storing the GroupKey on the Peer at Add time was essential for correctness.
- `group-updates` (existing per-peer config) and update groups (this feature) are orthogonal but confusingly named. The spec and docs clarify both.

## Files

- `internal/component/bgp/reactor/update_group.go` -- GroupKey, UpdateGroup, UpdateGroupIndex
- `internal/component/bgp/reactor/update_group_test.go` -- 11 unit tests
- `internal/component/bgp/reactor/reactor.go` -- updateGroups field
- `internal/component/bgp/reactor/reactor_notify.go` -- Add/Remove wiring
- `internal/component/bgp/reactor/reactor_api_batch.go` -- group-aware build
- `internal/component/bgp/reactor/reactor_api_forward.go` -- cache-aware forward
- `internal/component/bgp/reactor/peer.go` -- updateGroupKey field
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- YANG leaf
- `internal/component/config/environment.go` -- env var registration
- `internal/exabgp/migration/migrate.go` -- injection
- `test/parse/update-groups-disabled.ci` -- parse test
- `docs/features.md`, `docs/guide/configuration.md`, `docs/architecture/update-building.md`, `docs/comparison.md`
