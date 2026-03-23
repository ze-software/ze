# 415 -- prefix-data

## Context

Operators need prefix maximum values for every BGP peer but determining correct values manually is tedious. The predecessor spec (413-prefix-limit) implemented enforcement but deferred the data source. The original design proposed embedded routing data, zefs storage, and a build pipeline. The user simplified this to direct PeeringDB queries at runtime with per-peer staleness tracking.

## Decisions

- Query PeeringDB directly at runtime over building a data pipeline. Eliminates embedded JSON, zefs storage, build scripts, and source-url config.
- PeeringDB settings in `system { peeringdb { } }` over `bgp { }` because PeeringDB is an external service, not a BGP concept. Other subsystems could use it later.
- Per-peer hidden `updated` leaf over per-family timestamps because the update command refreshes all families for a peer at once.
- Staleness threshold fixed at 180 days over configurable because 6 months is universally reasonable and one fewer config knob.
- `isConnReset` simplified to `errors.Is(err, syscall.ECONNRESET)` over `errors.As` + `errors.Is` chain, matching the pattern in `runner_validate.go`.

## Consequences

- Operators can run `ze bgp peer * prefix update` to refresh prefix maximums from PeeringDB with configurable margin (default 10%).
- `ze bgp peer X detail` shows `prefix-updated` date and `prefix-stale: true` when data is older than 6 months.
- Prometheus: `ze_bgp_prefix_ratio` (count/maximum) and `ze_bgp_prefix_stale` (1 if stale) complement the 6 existing prefix metrics.
- Startup log warns for each peer with stale prefix data.
- `closeConn()` now does graceful TCP shutdown (CloseWrite + drain), fixing NOTIFICATION delivery reliability for all session teardowns, not just prefix limits.
- Enforcement .ci test (`prefix-maximum-enforce.ci`) unblocked by combining EOR expectation with stderr assertion.
- CLI login staleness banner not implemented -- deferred to general CLI login warning system (not prefix-data specific).

## Gotchas

- All session unit tests use `net.Pipe()` which bypasses `*net.TCPConn` type assertion. The CloseWrite/drain code path is only exercised by `TestCloseConnGracefulTCP` (real TCP) and the `.ci` functional tests.
- ze-peer sends routes immediately after OPEN handshake, but ze sends EOR first. Tests expecting NOTIFICATION from enforcement must expect EOR as seq=1 before NOTIFICATION as seq=2.
- `connectionEstablished()` sends an OPEN message, so tests that set up TCP sessions directly must use raw field assignment instead.
- `PeerInfo.PrefixUpdated` is populated from `PeerSettings.PrefixUpdated` in the reactor API adapter. The peer detail handler computes staleness inline (cannot import reactor package due to cycle risk).

## Files

- `internal/component/bgp/peeringdb/client.go` -- PeeringDB HTTP client
- `internal/component/bgp/peeringdb/client_test.go` -- 12 unit tests
- `cmd/ze-test/peeringdb.go` -- fake PeeringDB server (deterministic: ipv4=ASN, ipv6=ASN/5)
- `internal/component/bgp/plugins/cmd/peer/prefix_update.go` -- update command handler
- `internal/component/bgp/reactor/session_prefix.go` -- IsPrefixDataStale, prefix_ratio metric, staleness metric
- `internal/component/bgp/reactor/session_connection.go` -- graceful TCP close (CloseWrite + drain)
- `internal/component/bgp/reactor/reactor_metrics.go` -- prefix_ratio and prefix_stale gauges
- `internal/component/bgp/reactor/peersettings.go` -- PrefixUpdated field
- `internal/component/bgp/reactor/config.go` -- parse updated leaf
- `internal/component/config/system/system.go` -- PeeringDB URL/margin extraction
- `internal/component/config/system/schema/ze-system-conf.yang` -- peeringdb container
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- updated hidden leaf
- `internal/component/bgp/plugins/cmd/peer/schema/ze-peer-cmd.yang` -- prefix update RPC
- `internal/component/plugin/types.go` -- PeerInfo.PrefixUpdated field
- `test/plugin/api-peer-prefix-update.ci` -- update command functional test
- `test/plugin/prefix-stale-warning.ci` -- staleness warning functional test
- `test/plugin/prefix-maximum-enforce.ci` -- enforcement functional test
