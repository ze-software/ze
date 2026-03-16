# 223 — Config Reload 2: Coordinator

## Objective

Implement the reload coordinator that orchestrates the verify→apply two-phase commit across all registered plugins before updating the reactor.

## Decisions

- Import cycle between `internal/plugin` and `internal/config` prevents using `config.DiffMaps` — duplicated as a local `diffMaps` function in the coordinator. This is a deliberate trade-off: breaking the cycle would require moving `DiffMaps` to a shared package, which adds complexity.
- `TryLock()` (Go 1.18+) used for non-blocking concurrent reload rejection — a second reload arriving during an in-progress reload returns an error immediately rather than queuing.
- Root removal must send `"{}"` (empty JSON object) to plugins, not a silent skip — a plugin that receives no update for a root it previously configured cannot distinguish "no change" from "deleted", so the explicit empty object signals deletion.
- Wildcard `"*"` root required an explicit `wantsAll` flag on the plugin registration — the coordinator cannot match `"*"` using the same string comparison as named roots.

## Patterns

- Two-phase commit (verify-then-apply) for distributed config changes: verify that all parties accept the change before any party applies it. Failure in verify causes full abort.
- `TryLock` for coordinator exclusion is simpler than a queue and avoids reload storms during rapid config changes.

## Gotchas

- Root removal as a silent skip was the first implementation — it caused plugins to silently retain stale config. The empty-object signal is not obvious but is necessary for correct semantics.
- The import cycle is a recurring friction point: `internal/plugin` cannot import `internal/config` types, requiring either duplication or a shared package extraction.

## Files

- `internal/component/plugin/coordinator/` — reload coordinator implementation
- `internal/component/plugin/coordinator/diff.go` — local diffMaps (duplicated from config package)
