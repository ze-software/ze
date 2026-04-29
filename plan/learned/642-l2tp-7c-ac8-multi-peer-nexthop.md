# 642 -- L2TP Multi-Peer NEXT_HOP Test

## Context

The L2TP redistribute pipeline emits `nhop self` for subscriber routes,
and the reactor's `resolveNextHop()` substitutes each peer's own
`LocalAddress`. The single-peer test (`redistribute-l2tp-announce.ci`)
proved the path for one peer but did not verify that two peers with
different local addresses get distinct NEXT_HOP values. AC-8 from
spec-l2tp-7c-redistribute required this coverage.

## Decisions

- Two ze-peer processes sharing `$PORT` but binding to different loopback
  addresses (127.0.0.1 and 127.0.0.2), using `option=bind:value=` and
  `--bind` CLI flag, over allocating a second port variable.
- Both peers are iBGP (same local/remote ASN 65533), since the NEXT_HOP
  resolution path is the same for iBGP and eBGP when `nhop self` is used.
- Python observer reuses the fakel2tp dispatch-command pattern from the
  single-peer test, with a longer initial sleep (3s) to allow both
  sessions to establish.

## Consequences

- AC-8 from spec-l2tp-7c-redistribute is now fully covered.
- The two-peer loopback pattern (`--bind 127.0.0.2`) is a reusable
  template for future multi-peer functional tests.

## Gotchas

- The `option=bind:value=` directive in ze-peer stdin blocks is not
  documented in `--help` output; it is parsed from the stdin option
  protocol, not the CLI flags.
- Pre-existing enum-over-string breakage (Phase 8 commit) required
  fixing `rpc.SessionState` string comparisons across 8 files before
  `make ze-verify` could pass.

## Files

- `test/plugin/redistribute-l2tp-multi-peer-nexthop.ci` (created)
- `internal/component/bgp/plugins/rib/rib.go` (enum fix)
- `internal/component/bgp/plugins/rib/rib_bestchange_test.go` (enum fix)
- `internal/component/bgp/plugins/bmp/bmp.go` (enum fix)
- `internal/component/bgp/plugins/bmp/event_test.go` (enum fix)
- `internal/component/bgp/plugins/rr/rr.go` (enum fix)
- `internal/component/bgp/plugins/gr/gr.go` (enum fix)
- `internal/component/bgp/plugins/persist/server.go` (enum fix)
- `internal/component/bgp/plugins/watchdog/watchdog.go` (enum fix)
- `internal/component/bgp/plugins/adj_rib_in/rib.go` (enum fix)
- `internal/component/bgp/plugins/rs/server.go` (enum fix)
- `internal/component/bgp/reactor/reactor_api.go` (enum fix)
- `internal/component/bgp/server/events_test.go` (enum fix)
- `internal/component/plugin/server/benchmark_test.go` (enum fix)
