# 1079 -- unify-config-diff

## Context

Package `internal/component/plugin/server` carried a private copy of the deep
map-diff algorithm (`configDiff`/`diffPair`/`diffMaps`/`diffMapsRecursive`/`diffJoinPath`
in `reload.go`), structurally identical to the canonical `config.DiffMaps` in
`internal/component/config/diff.go`. The copy was justified by a comment claiming an
import cycle "with internal/config". DESIGN-REVIEW.md finding 2 flagged the duplication.
Goal: delete the duplicate and route `server` through `config.DiffMaps`, preserving the
plugin JSON payloads and CLI diff output byte-for-byte.

## Decisions

- Reused `config.DiffMaps` directly over relocating it to a new `internal/core/` leaf:
  `plugin/server` already imports `config` (`reload.go` for `AppendPath`/`PathSep`/
  `ExtractConfigSubtree`), so the call adds zero new package edge. The leaf-package move
  would force `config` + its `ze config diff` CLI consumer to migrate for no coupling win.
- Kept `config.DiffMaps` where it lives (package `config`) over moving it: `config` parses
  the trees being diffed and is the natural owner; both consumers already sit above/beside it.
- Deleted `TestDiffMapsLocal` over repointing it at `config.DiffMaps`: it would exactly
  duplicate `config/diff_test.go` (12 tests), which already covers the surviving impl.

## Consequences

- One diff implementation remains (`config.DiffMaps`); `server`'s `rootHasChanges`,
  `buildDiffSections`, `runTxCoordinator`, `buildTxInputs` now take `*config.ConfigDiff`.
- The `server -> config` edge stays one-directional; `dep_audit.py` (`make ze-tier-check`)
  guards against a future `config -> server` regression that would break the direct call.
- JSON parity is load-bearing and preserved: `config.DiffPair` and the deleted `diffPair`
  share json tags `old`/`new`, so `rpc.ConfigDiffSection.Changed` payloads are byte-identical.

## Gotchas

- The "import cycle" comment was a fossil: it referenced `internal/config`, a path that no
  longer exists (the package is `internal/component/config`), and the file already compiled
  while importing that package. A stale defensive comment can freeze a duplication in place
  long after its constraint dissolved. Verifying the constraint (one grep + the fact that the
  file already imports+compiles against `config`) is cheaper than trusting the comment.
- `grep -rl plugin/server internal/component/config` is NOT empty, but the matches are CLI
  *subpackages* (`config/archive/cmd`, `config/yang/cli`, `config/schema/cli`) that sit above
  both packages in the tier graph -- Go packages are per-directory, so importing a config
  *subpackage* is not the top-level `config` package importing `server`. No cycle.
- `ze-validate` re-scans the WHOLE changed file, so touching `reload.go` surfaced 4
  pre-existing "no cross-package caller" findings on unrelated Server API methods
  (`TxLocked`/`FullReloadFunc`/`HasFullReloadFunc`/`ReloadFull`). They predate the diff and
  `ze-validate` is a post-verify advisory (not in the commit gate) -- do not "fix" them here.

## Files

- `internal/component/plugin/server/reload.go` -- deleted the private diff types+funcs;
  producer now `config.DiffMaps`; consumers retyped to `*config.ConfigDiff`; removed unused
  `reflect` import.
- `internal/component/plugin/server/reload_tx.go` -- retyped `runTxCoordinator`/`buildTxInputs`
  to `*config.ConfigDiff`; added top-level `config` import.
- `internal/component/plugin/server/reload_test.go` -- migrated `TestRootHasChanges`,
  `TestDiffPairJSONKeys`, `TestBuildDiffSections` to `config.ConfigDiff`/`config.DiffPair`;
  deleted `TestDiffMapsLocal` (`test-relax:` marker; superseded by `config/diff_test.go`).
