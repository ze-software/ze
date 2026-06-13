# 892 -- Post-Implementation Validation Tool

## Context

Review cycles were catching the same classes of mistakes repeatedly: stale source anchors in docs, line-number anchors that rot on edit, exported symbols wired only to tests (not production entry points), and spec audit tables left unfilled. These are Python-checkable patterns that `ze-verify` (Go lint + tests) cannot catch. `make ze-validate` fills that gap as a fast post-verify pre-screen.

## Decisions

- Chose a standalone Python script over extending `verify_wiring_docs.py` because the checks are conceptually different (doc/spec hygiene vs wiring correctness) and the existing script already routes to multiple make targets.
- Chose per-symbol `grep -rlw` over Python `rglob` + `read_text` for the cross-package wiring check because grep is an order of magnitude faster on large trees (0.2s total vs estimated 2-5s).
- Skipped `.ci` functional test over creating one because the `.ci` format is for BGP protocol testing via `ze-test`; a Makefile-target test does not fit that execution model. Coverage is via unit tests in `validate_test.py`.
- Added URL and no-slash filtering to the stale-path check over strict path validation because the codebase uses `<!-- source: https://... -->` for external references and docs contain inline `<!-- source: ... -->` examples that would false-positive.

## Consequences

- Sessions can run `make ze-validate` after `make ze-verify` to catch doc/spec hygiene issues before presenting work as complete.
- New check categories can be added by writing a `check_*` function and adding it to `run_checks()`.
- The cross-package wiring check in `ze-validate` overlaps with `verify_wiring_docs.py`'s wiring gate; the two are complementary (different triggers, same concept).

## Gotchas

- Source anchors in inline table cells (documentation-testing.md describes the anchor format and uses literal `<!-- source: ... -->` examples) are parsed by the regex and produce false positives unless paths without `/` are skipped.
- `grep -rlw --include=*.go` on macOS requires the glob pattern without quotes in the subprocess list (Python's `subprocess.run` with list args does not shell-expand, so `--include=*.go` works correctly).

## Files

- `scripts/dev/validate.py` (created) -- validation script with 5 check functions
- `scripts/dev/validate_test.py` (created) -- 11 unit tests
- `Makefile` (modified) -- `ze-validate` target, `.PHONY`, help text
- `docs/functional-tests.md` (modified) -- documented ze-validate as post-verify check
