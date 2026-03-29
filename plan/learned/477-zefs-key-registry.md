# 477 -- ZeFS Key Registry

## Context

ZeFS blob keys (`meta/ssh/username`, `meta/history/{user}/{mode}`, `file/active/{name}`) were hardcoded string literals scattered across 11+ consumer files. There was no way to discover all known keys programmatically or from the CLI. The `env` package already solved this for environment variables with `MustRegister()`. The goal was to replicate that pattern for zefs keys, adding template support for keys with variable segments.

## Decisions

- Chose centralized registration in `pkg/zefs/` (bottom of import graph) over scattered per-package registration, because consumers like `cmd/ze/init/` and `cmd/ze/internal/ssh/client/` can't import each other to share constants.
- Used `{param}` curly-brace placeholders over `<param>` angle brackets (used by env), because curly braces are more standard for template substitution.
- No enforcement on BlobStore access (unlike env which aborts on unregistered `Get()`). The registry is for documentation and CLI discoverability only. Adding enforcement would require changing every `ReadFile`/`WriteFile` call site.
- Added validation in `Key()` (reject empty params and `..`) and `MustRegister` (reject empty patterns and duplicates) after deep review identified path traversal and misconfiguration risks.
- Private keys (password, web cert/key) marked `Private: true` and excluded from `ze data registered` listing and single-key lookup.
- Consumer files use `zefs.Key*.Pattern` for fixed keys and `zefs.Key*.Key(param)` for templates. Local constants replaced, not layered.
- `const` to `var` for values derived from registry (e.g., `grmarker.markerKey`, `history.historyKeyPrefix`) since Go requires const values to be compile-time literals.

- Unified CLI pattern: both `ze env` and `ze data` now use `registered` as the subcommand to list/inspect registered entries. Bare `ze env` (previously listed all vars) now shows usage instead, matching `ze data` behavior. `ze env list [-v]` kept as alias for backwards compatibility.

## Consequences

- All 13 zefs key patterns are now discoverable via `ze data registered` CLI command.
- All env vars discoverable via `ze env registered`. Both commands support `registered <key>` for single-key detail.
- Future keys must be registered in `pkg/zefs/keys.go` -- the `TestAllProductionKeysRegistered` test catches missing registrations.
- `Key()` validates params at call time (empty and `..` rejected with panic), providing defense-in-depth beyond BlobStore's `fs.ValidPath()` check.
- Template keys like `KeyHistory.Key(username, mode)` provide type-safe key construction, replacing error-prone string concatenation.

## Gotchas

- Worktree agent ran for 50 minutes but failed to commit its work. Had to re-implement in the main repo. The worktree also picked up unrelated changes from other branches.
- The auto-linter hook removes unused imports immediately, so adding an import and its usage must happen in the same edit. Adding `tabwriter` import before adding `cmdRegistered` caused cascading edit failures.
- `MustRegister` tests must save/restore global state and must never use `t.Parallel()` since they mutate package-level slices.
- Deep review found `showRegisteredKey` was using `AllEntries()` (including private), bypassing the intent of the `Private` flag. Fixed to use `Entries()`.

## Files

- `pkg/zefs/registry.go` -- KeyEntry, MustRegister, Entries, AllEntries, IsRegistered
- `pkg/zefs/keys.go` -- 13 key registrations
- `pkg/zefs/registry_test.go` -- 17 tests
- `cmd/ze/data/main.go` -- `ze data registered` subcommand
- `cmd/ze/environ/main.go` -- `ze env registered` subcommand, bare `ze env` now shows usage
- 11 consumer files migrated from hardcoded strings to registry references
