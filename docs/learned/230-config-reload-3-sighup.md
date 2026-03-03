# 230 — Config Reload: SIGHUP Wiring

## Objective
Wire SIGHUP through a reload coordinator and split `reactor.Reload()` into separate `VerifyConfig()` + `ApplyConfigDiff()` methods that the coordinator can call independently.

## Decisions
- `HasConfigLoader()` guard on SIGHUP handler: coordinator path only activates when a config loader is wired. Without the guard, every production SIGHUP silently failed.
- `loadPeersFullOrTree()` prefers `reloadFunc` (full config pipeline, all 16 fields) over `parsePeersFromTree` (6 fields). Using the partial parser caused false "peer changed" diffs on every reload.
- `Reload()` kept as backward-compatible wrapper calling `reconcilePeers` directly — it does NOT delegate to `VerifyConfig + ApplyConfigDiff` because its input source is a file path, not a tree.
- Double file read in coordinator path (ConfigLoader + reloadFunc) is a conscious correctness-over-efficiency tradeoff; reloads are rare.
- Functional tests (SIGHUP + full session) deferred — require daemon orchestration; covered in spec 234.

## Patterns
- `reconcilePeers(newPeers, label)` extracts shared diff/add/remove logic: build new map, stop removed, add new. Called by both `Reload()` and `ApplyConfigDiff()`.
- Verify is side-effect-free (read-only); apply executes mutations. Ordering in coordinator does not affect correctness.

## Gotchas
- `r.api != nil` is always true in production, so checking the interface alone is not sufficient — a dedicated `HasConfigLoader()` predicate is needed.
- `parsePeersFromTree` is a test-only fallback; never use it in production paths that call `peerSettingsEqual`.

## Files
- `internal/plugin/bgp/reactor/reactor.go` — VerifyConfig, ApplyConfigDiff, reconcilePeers, loadPeersFullOrTree, SIGHUP guard
- `internal/plugin/reload.go` — HasConfigLoader, SetConfigLoader, ReloadFromDisk, reloadConfig
- `internal/plugin/types.go` — ReactorInterface extended with VerifyConfig/ApplyConfigDiff
