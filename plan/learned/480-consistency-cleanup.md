# 480 -- Consistency Cleanup

## Context

Codebase consistency audit identified mechanical issues across 6 categories: test variable naming (`tc` vs `tt`), missing `doc.go` files, inline `errors.New()`, hyphenated filenames, error wrapping (`%v`/`%s` vs `%w`), flaky `time.Sleep` tests, hyphenated plugin directories, `t.Parallel()` adoption, and `ChunkMPNLRI` append usage. Work was split into 6 phases by risk level, with phases 1-4 implemented across multiple prior sessions.

## Decisions

- Phases 1-4 done in prior commits (78481a08, 3a9bb69b, and others): `tc`->`tt`, `doc.go` files, sentinels, file renames, error wrapping audit, flaky test fixes, directory renames to underscores.
- Phase 5 (`t.Parallel()`) dropped -- adoption is already high (2681 occurrences), full per-package audit has diminishing returns.
- Phase 6 (`ChunkMPNLRI` buffer-first) dropped -- the `append` builds a `[][]byte` index of zero-copy subslices, not wire bytes. Does not violate buffer-first rule. AC-9 was a false positive.
- Plugin directory convention: underscores chosen over concatenated or hyphens (e.g., `adj_rib_in`, `route_refresh`).

## Consequences

- All error wrapping in project code uses `%w` where the argument is an `error` type.
- All plugin directories under `bgp/plugins/` use underscores consistently.
- All table-driven test loops use `tt` not `tc`.
- No dedicated `ChunkMPNLRI` spec needed -- the code is already correct.

## Gotchas

- The original audit counted ~385 `fmt.Errorf` candidates for `%w`, but most were string-typed args (`%s` with filenames, commands, etc.). Actual error-wrapping instances were far fewer.
- `ChunkMPNLRI` append looked like a buffer-first violation but is actually building a slice-of-subslices index (zero-copy). The buffer-first rule targets wire byte assembly, not control-plane metadata.
- New files added after the original spec (e.g., `pkg/zefs/`, `internal/component/web/`) introduced fresh `tc` instances that needed cleanup.

## Files

- `pkg/zefs/lock_test.go` -- `tc`->`tt`
- `pkg/zefs/store_test.go` -- `tc`->`tt`
- `internal/component/web/sse_test.go` -- `tc`->`tt`
