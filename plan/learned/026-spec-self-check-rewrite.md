# 026 — Self-Check Rewrite

## Objective

Replace the monolithic `test/cmd/self-check` test runner with a modular architecture modelled on ExaBGP's `qa/bin/functional`, adding per-test state machines, timing caches, and richer CLI options.

## Decisions

- Chose Go (not Python) for the new runner to stay consistent with the codebase.
- Nick assignment (0-9, A-Z, a-z) is deterministic (sorted by name), so test IDs are stable across runs.
- Timing cache stored at `~/.cache/zebgp/test_times.json` using a 5-run rolling average for ETA.

## Patterns

- Test types (encoding, API) share infrastructure but differ in execution model: encoding uses server/client split, API uses socket path + `.run` script.
- `Record` holds all metadata; `Exec` wraps process lifecycle; `Tests` is the container. This separation mirrors ExaBGP's class hierarchy.

## Gotchas

- The old `self-check` was 705 lines all-in-one; the replacement spans multiple packages. Preserving backward compat (`make self-check`) required aliasing.
- ExaBGP's timing features (psutil-based cleanup, stress modes) had no direct Go equivalent — had to re-implement from scratch.

## Files

- `test/cmd/functional/main.go` — new runner entry point
- `internal/test/runner/` — shared test infrastructure (eventually renamed from `test/selfcheck/`)
