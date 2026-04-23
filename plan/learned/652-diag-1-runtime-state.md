# 652 -- diag-1-runtime-state

## Context

Ze had rich internal runtime state (L2TP observer event rings, CQM echo buckets,
reliable transport window state, plugin lifecycle, traffic control qdiscs, BGP
attribute pool metrics) but no CLI commands to query it. An AI assistant connected
via MCP could see tunnel/session/peer state but could not inspect echo quality,
FSM event history, transport window behavior, or pool occupancy. The goal was to
expose all existing internal state through CLI commands, which auto-surface as MCP
tools via YANG RPC registration.

## Decisions

- Chose per-domain command handlers (Approach A) over a single diagnostic module
  (Approach B), because Approach B would break component isolation by importing
  from L2TP, BGP, traffic, and plugin packages in one place.
- Added methods to Observer (`SessionSummaries`, `LoginSummaries`, `EchoState`)
  rather than exposing the raw `eventRing`/`sampleRing` types, preserving the
  existing encapsulation.
- Changed AC-6 from session-id to login-name for echo queries, because CQM data
  is keyed by login, not session. No per-session echo state exists.
- Put traffic show in `cmd/show/` (alongside `show interface`) rather than a new
  `cmd/traffic/` package, avoiding a single-handler package.
- Put pool stats in `cmd/metrics/` as `metrics pool` rather than `show bgp pool`,
  since attribute pools are BGP internals adjacent to the existing metrics surface.

## Consequences

- All new commands are auto-exposed as MCP tools for AI-assisted troubleshooting.
- Claude can now diagnose L2TP session flaps (observer event ring), echo quality
  degradation (CQM buckets), transport issues (reliable window), and pool pressure
  (dedup rates) without stopping the daemon.
- The `Service` interface grew by 4 methods; existing fake implementations in tests
  needed updating. Future Service additions require the same maintenance.

## Gotchas

- `show interface` already existed with brief/errors/counters/type variants. The
  umbrella spec assumed it didn't. Always verify "does not exist" claims during
  child spec RESEARCH.
- `subsystem-list` was hardcoded to return `["bgp"]`. The stub was so plausible
  that no one noticed it wasn't querying real state until this spec.
- `totalRespawns` on ProcessManager is unexported with no getter. Plugin restart
  count is not yet exposed; would need a public method added.

## Files

- `internal/component/l2tp/observer.go` -- SessionSummaries, LoginSummaries, EchoState
- `internal/component/l2tp/reliable.go` -- ReliableStats type, Stats() method
- `internal/component/l2tp/snapshot.go` -- ReliableStats on L2TPReactor
- `internal/component/l2tp/subsystem_snapshot.go` -- facade methods
- `internal/component/l2tp/service_locator.go` -- Service interface extension
- `internal/component/cmd/l2tp/l2tp.go` -- 4 new handlers
- `internal/component/l2tp/schema/ze-l2tp-api.yang` -- 4 new RPCs
- `internal/component/cmd/l2tp/schema/ze-l2tp-cmd.yang` -- show tree augments
- `internal/component/plugin/server/system.go` -- real subsystem-list
- `internal/component/cmd/show/show.go` -- warning/error filters, traffic show
- `internal/component/cmd/metrics/metrics.go` -- pool stats handler
