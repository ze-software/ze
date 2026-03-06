# 371 — Route Server (bgp-rs) & Persist (bgp-persist) Plugins

## Objective

Retroactive spec audit of the RS and Persist plugins — both fully implemented before
the spec was created. Fixed persist EOR bug discovered during review.

## Decisions

- Forward-all model for RS: no best-path selection, zero-copy cache forward
- Persist keeps ribOut on peer down (routes were sent, replay on reconnect)
- RS and persist are separate plugins — shared abstraction deferred until 3+ users
- Multi-peer forwarding functional tests deferred: test framework supports single $PORT only
- Worker pool per-source-peer with backpressure (pause/resume RPCs) for RS forwarding

## Patterns

- `updateRoute(peer, cmd)` with test hook: nil in production, captures (peer, cmd) pairs in tests
- Generation guard (`replayGen uint64`) on replay goroutines prevents stale goroutines from rapid reconnect
- Convergent delta replay: full replay → delta loop → EOR. Handles race between event delivery and replay snapshot
- Async cache release via buffered channel with fallback sync RPC on overflow

## Gotchas

- **Persist EOR format was wrong:** used `bgp eor <family> <peer>` which is not a registered engine command.
  Correct format is `update text nlri <family> eor` (same as RS). Test only checked `Contains("eor")` so
  it didn't catch the bug — strengthened test to validate `update text nlri` prefix.
- **Spec file paths stale:** spec referenced `internal/plugins/bgp-rs/` but actual code at
  `internal/component/bgp/plugins/bgp-rs/` — path restructuring happened after spec was written.
- **Test framework limitation:** single $PORT per .ci test. Multi-peer scenarios (A→B forwarding)
  would require framework extension ($PORT2, multiple ze-peer instances).

## Files

- `internal/component/bgp/plugins/bgp-rs/` — RS plugin (server, handlers, forward, withdrawal, text, worker, peer)
- `internal/component/bgp/plugins/bgp-persist/server.go` — Persist plugin (fixed sendEOR)
- `internal/component/bgp/plugins/bgp-persist/server_test.go` — Added EOR format assertion
- `test/plugin/plugin-persist-features.ci` — New functional test
