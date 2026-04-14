# 591 -- Multipath / ECMP

## Context

Ze supported only single best-path selection per RFC 4271. Network operators deploying ECMP need multiple equal-cost paths per prefix for load balancing. The `bgp/multipath` container with `maximum-paths` and `relax-as-path` was needed as a global config knob to extend the RIB plugin's best-path selection.

## Decisions

- Global config (`bgp/multipath`) over per-peer, because multipath is a RIB-level decision that applies to all prefixes regardless of source peer.
- Post-selection approach: `SelectMultipath` runs AFTER `SelectBest` picks the single winner, then scans remaining candidates for equal-cost matches. This preserves the existing single-best codepath untouched (maximum-paths=1 is a no-op).
- Equal-cost gates: LOCAL_PREF, AS_PATH length (+ content unless relaxed), Origin, MED (same neighbor AS), eBGP/iBGP status. Steps 0 (stale), 6 (IGP cost), 7 (router-id), 8 (peer-address) excluded as tiebreakers.
- Config delivery via Stage 2 `OnConfigure` callback with atomic fields (`maximumPaths`, `relaxASPath`), same pattern as GR plugin's restart-time.
- Output as `"multipath-peers"` array in `rib best` JSON (omitempty when single-best), over a separate `rib multipath` command.

## Consequences

- FIB/ECMP consumers can iterate `[primary] + siblings` for nexthop group programming.
- `rib best` output transparently shows multipath when configured, no new command needed.
- The `relax-as-path` flag enables Cisco-style `as-path multipath-relax` behavior.
- ADD-PATH interaction is natural: multipath selects N best from the full candidate set regardless of how they arrived.

## Gotchas

- The spec was severely stale: claimed Phase 1/3 (YANG only) when all three phases were fully implemented with 20+ unit tests and pipeline integration tests. Discovered during 2026-04-14 audit.
- `extractMultipathConfig` handles three JSON numeric formats (float64, int, string) because the config pipeline doesn't guarantee a single type for uint16 values.
- The functional .ci test (`multipath-basic.ci`) uses `bgp rib inject` to create a synthetic second peer's route, avoiding the two-IP ze-peer connection issue on macOS.

## Files

- `internal/component/bgp/schema/ze-bgp-conf.yang` -- multipath container
- `internal/component/bgp/plugins/rib/bestpath.go` -- SelectMultipath, multipathEqual
- `internal/component/bgp/plugins/rib/rib_multipath_config.go` -- extractMultipathConfig
- `internal/component/bgp/plugins/rib/rib.go` -- OnConfigure wiring, atomic fields
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` -- MultipathPeers in pipeline
- `test/parse/multipath-config.ci`, `test/plugin/multipath-basic.ci`
