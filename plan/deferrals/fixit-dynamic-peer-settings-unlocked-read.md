# Deferrals: fixit-dynamic-peer-settings-unlocked-read

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** the dynamic-peer resolver reads the shared settings struct unlocked, on a sole-writer claim the reload swap falsified

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | ad-hoc (found by the race test written for the reload-path unlocked read, `TestReloadDecisionReadsPeerSettingsUnderLock`) | `resolveDynamicPeerSettings` (`internal/component/bgp/reactor/reactor_dynamic.go`) reads `p.settings.ImportFilters` and `p.settings.ExportFilters` with no lock, to capture the unresolved templates into `p.dynImportFilters` / `p.dynExportFilters`. Its comment states the reason: "These reads and the dyn* template capture run on the establishment goroutine, the only writer of these fields, so they need no lock here." That claim was true when written and is now FALSE: `applyHotSwappableSettings` (`peer_settings_apply.go`) writes both fields from the RELOAD goroutine under `p.mu`, through `hotSwappableSettings`. The race detector reports it directly, as a read in `resolveDynamicPeerSettings` against a write in `hotSwappableSettings`. The consequence is not only a torn slice header: the template capture can latch the RELOADED filter chain as the "original unresolved template", so every later reconnection of that dynamic peer resolves `$remote_as` / `$remote_ip` against a chain that was already resolved once | Separable from the reload-path read that the negotiation-equivalence review named, which is fixed: that one is a whole-struct read in `reconcilePeersJournaled` (`reactor_api.go`) and is now taken through `Peer.SettingsSnapshot` (`peer.go`). This one is the OTHER side of the same pair, in a different file, on the establishment goroutine, and it needs its own decision -- taking `p.mu` around the template capture is not a one-line move, because `resolveFilterVars` allocates and the existing code deliberately runs it outside the lock. `internal/component/bgp/reactor/reactor_dynamic.go` was outside the owned file set for that work | needs a destination spec | done |

## Reproduction

`TestReloadDecisionReadsPeerSettingsUnderLock`
(`internal/component/bgp/reactor/peer_settings_negotiation_test.go`) reproduces it
when its concurrent writer is `peer.resolveDynamicPeerSettings(session)` instead of
`peer.applyHotSwappableSettings(other, hotSwappableSettings)`. Run under
`go test -race ./internal/component/bgp/reactor/...`. The committed form of that test uses the locked writer on
purpose, so it reports the read it was written for rather than this one.

Observed, with the dynamic writer in place:

```
WARNING: DATA RACE
Read at 0x00c0001b8b50 by goroutine 38:
  ...reactor.(*Peer).resolveDynamicPeerSettings()
Previous write at 0x00c0001b8b50 by goroutine 37:
  ...reactor.hotSwappableSettings()
  ...reactor.(*Peer).applyHotSwappableSettings()
```

## Next step

Decide whether the template capture moves under `p.mu` (with the allocation kept
outside it, as the current comment intends) or whether the `dyn*` templates are
captured once at peer construction, where no reload can have run yet. Then correct
the stale sole-writer comment either way (`ai/rules/stale-comments.md`).


Closed 2026-08-29 after verifying the producer rather than the row: `(*Peer).resolveDynamicPeerSettings` (`internal/component/bgp/reactor/reactor_dynamic.go`) now takes `session.mu.RLock()` around the read, verified in the tree.
