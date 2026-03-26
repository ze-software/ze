# 429 -- prefix-limit

## Context

BGP peers can send an unbounded number of prefixes, exhausting memory and masking route leaks. Every major BGP implementation supports per-peer prefix limits. Ze needed this both as a safety mechanism and as a prerequisite for `spec-forward-congestion` (prefix maximum sizes per-peer buffer allocations). The feature was built across three phases: enforcement (413), PeeringDB data source (415), and warnings/show verb (this session).

## Decisions

- Mandatory per-family prefix maximum over optional, because an unconfigured peer defeats the safety purpose.
- Enforcement before plugin delivery over post-delivery, so over-limit routes never reach the RIB.
- Direct PeeringDB queries at runtime over embedded routing data pipeline (zefs, build scripts, source-url). Eliminates complexity for equivalent operator experience.
- Login banner shows 1 warning in detail, collapses to "N warnings" for multiple, over always showing a count. Operators with a single problem see exactly what it is.
- `show` verb package (`show bgp peer X`, `show bgp warnings`) over noun-first syntax (`peer X detail`), matching the verb-first pattern of `update/set/del`. The noun-first `peer X detail` RPC still exists for internal dispatch.
- Prefix warning state tracked on Peer via mutex-protected map with session callback over reading session state directly (race) or adding atomics (insufficient detail for API).
- `isPrefixStale` duplicated in peer_warnings.go (6 lines) over importing reactor (import cycle).

## Consequences

- Every `.ci` test and chaos config must include `prefix { maximum N; }` for each family (176+ files affected by 413).
- `show` verb is now available for future read-only introspection commands beyond peer detail and warnings.
- `PeerInfo.PrefixWarnings` field available for any consumer that queries reactor peers.
- Staleness threshold (180 days) is hardcoded. If it ever needs to be configurable, change in two places: `reactor/session_prefix.go` and `plugins/cmd/peer/peer_warnings.go`.

## Gotchas

- YANG `family` was already a `list`, not a `leaf-list`. The original spec incorrectly assumed a migration was needed.
- Config parsing is in `reactor/config.go`, not `config/peers.go` (which does not exist).
- `plugins/cmd/peer` cannot import `reactor` (import cycle). Any shared logic like `IsPrefixDataStale` must be duplicated or extracted to a shared package.
- Test asserting "no NOTIFICATION" does not prove "routes rejected". AC-linked tests must assert behavior, not mechanism absence.
- `time.DateOnly` format loses time-of-day, so staleness boundary tests at exactly 180 days depend on time-of-day offset.

## Files

- `internal/component/bgp/reactor/peer.go` -- prefixWarnedMap tracking, SetPrefixWarned/PrefixWarnedFamilies/clearPrefixWarned
- `internal/component/bgp/reactor/session.go` -- prefixWarningNotifier callback
- `internal/component/bgp/reactor/session_prefix.go` -- notifier calls on warning state changes
- `internal/component/bgp/reactor/reactor_api.go` -- PrefixWarnings populated in Peers()
- `internal/component/plugin/types.go` -- PeerInfo.PrefixWarnings field
- `internal/component/bgp/config/loader.go` -- collectPrefixWarnings for login banner
- `internal/component/bgp/plugins/cmd/peer/peer_warnings.go` -- HandleBgpWarnings RPC handler
- `internal/component/cmd/show/` -- show verb package (YANG + RPC registration)
