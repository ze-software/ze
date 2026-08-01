# 1148 — filter-irr: decouple startup from IRR resolution

## Context
Fix a flaky filter-irr functional test (#161) where an UPDATE arriving before the
first IRR resolution was rejected `no-prefix-list` instead of `modify` — WITHOUT
coupling ze startup to IRR-server reachability.

## Decisions
- First attempt (WRONG): make the initial IRR resolution SYNCHRONOUS inside
  `OnConfigure`. It fixed the race but coupled startup to the network. filter-irr
  resolves every configured BGP peer's ASN, defaulting to the public RADB server
  (`198.108.0.18:43`) when no server is configured. When RADB is unreachable
  (CI/offline), the synchronous lookup overran the plugin stage barrier, failed
  the configure stage, cascade-cancelled every bgp plugin, and ze exited 1 for
  ANY config with a peer (broke authz/ssh tests intermittently).
- Correct fix: keep the first resolution ASYNC (startup never blocks on IRR), and
  gate the race at the FILTER layer. Each ASN state carries a `firstDone` channel
  closed once on first resolution; `handleFilterUpdate` waits on it (bounded,
  `firstResolveWait = 5s`) only when the list is not yet populated, then re-checks.

## Patterns
- Gate a "data not ready yet" race at the CONSUMER (the filter handler), never by
  blocking a plugin's configure/startup on network I/O.
- Distinguish "first resolution not done yet" (wait, then re-check) from
  "resolution done and genuinely empty" (unchanged fail-closed reject).

## Gotchas
- A synchronous network call in `OnConfigure` can exceed the engine stage barrier
  (`defaultStageTimeout = 5s`) and cancel ALL plugins — startup must stay async.
- The on-disk cache (`database.zefs`) can pre-populate the list and mask the race
  non-deterministically; `loadFromStore` must NOT signal `firstDone` (cache is a
  fallback, used only if the live resolution wait times out).

## Files
- `internal/component/bgp/plugins/filter_irr/{filter_irr.go,cache.go,filter_irr_test.go}`.
