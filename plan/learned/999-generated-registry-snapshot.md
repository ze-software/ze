# 999 -- plugin/all registry snapshots: golden files, not hand-maintained lists

## Context

`internal/component/plugin/all/all_test.go` snapshot-tests the registered plugin
set: plugin names, RPC wire methods, YANG providers. These were hand-typed
`expected := []string{...}` literals that mirror the generated `all.go`. They
drifted: geodns was added to `all.go` (universal plugin) but never to the test
lists, so `TestRegisteredPluginNames` / `WireMethods` / `YANGSchemaProviders`
failed with "unexpected: geodns" under the feature-tagged build. The deeper
problem: a hand-maintained mirror of a generated/registry fact always drifts,
and adding a plugin needed a manual second step that was easy to forget AND only
caught by the full `make ze-unit-test` (not plain `go test`).

## Decisions

- **Snapshots are golden files, regenerated from the live registry.** The three
  lists moved to `testdata/{plugins,wire-methods,yang-providers}.snapshot`. A
  `snapshot(t, name, got)` helper compares the live registry to the golden, or
  rewrites it under `-update`. Regenerate with `make ze-plugin-snapshot`
  (`go test -tags '<ze_core + features>' -update ...`) and review the diff. The
  `assertSnapshot` comparison is unchanged -- only the source of `expected`
  moved -- so drift is still caught, but the fix is one command, not hand-editing
  sorted lists.
- **Golden via `go test -update`, NOT via the `plugin_imports.go` generator.**
  Extending the generator to emit the lists was the first idea, but many plugins
  register `Name: Name` (a package constant), not a string literal, so statically
  extracting names from `register.go` text is fragile (needs const resolution).
  The live registry is the reliable source, and `-update` reads it directly.

## Consequences

- Adding a plugin now: `make generate` (all.go) + `make ze-plugin-snapshot`
  (golden) + review both diffs. If you forget the snapshot, `ze-unit-test` fails
  with "unexpected: <plugin>" and the failure message points at the target, so it
  cannot silently pass.
- The snapshot represents the full-feature build (`ze_core` + all gates), so the
  test only matches under that build -- same as the old hardcoded lists. Under
  plain `go test` (no tags) it fails "missing isis/ldp/ospf/rsvp-te" by design.

## Gotchas

- Seeded the golden by extracting the prior (corrected) lists from the test
  source, because the tree was temporarily un-buildable under feature tags (an
  unrelated in-progress `iface/cmd` refactor with duplicate declarations), so
  `-update` could not be run live. Once that clears, `-update` reproduces the
  identical golden.
- General rule: a test that hard-codes a list mirroring a generated or
  registry-derived fact WILL drift. Generate it, or compare against the live
  source. (Wire methods and YANG providers also originate in YANG declarations
  and could additionally be cross-checked against the loaded YANG.)

## Files

- `internal/component/plugin/all/all_test.go` -- `snapshot()` helper, `-update`
- `internal/component/plugin/all/testdata/*.snapshot` -- golden files
- `Makefile` -- `ze-plugin-snapshot` target
