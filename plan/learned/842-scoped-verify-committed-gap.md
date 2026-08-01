# 842 -- scoped-verify-committed-gap

## Context

`make ze-verify-changed` (the scoped, parallel-safe verify variant) derived its
package set from the working-tree diff alone: `git diff` + `git diff --cached` +
`git ls-files --others`, duplicated inline in the `ze-lint-changed` and
`ze-unit-test-changed` Makefile targets and in `scripts/dev/changed-groups.sh`.

That set is empty for a package whose change has already been committed. So the
sequence "verify green -> edit a package -> commit -> run scoped verify on the
now-clean tree" tests *nothing* in that package and reports green even when the
committed package's tests are red. Commit `c148c9e80` hit exactly this: it
rewrote `web/testing` `Browser.WaitLoad()` without updating its golden test, the
package's unit tests went red, and the red commit landed. Only full
`make ze-verify` (which runs every package via `ze-unit-test-cached`) surfaced
it. Handover: `handover/15-web-testing-golden-regression.md`, "Open to
investigate" item 1.

## Decisions

- Closed the gap in the scoped path only. Full `ze-verify` already runs all unit
  tests cached, so its correctness never depended on the changed set; widening
  the full-verify `-race` pass (`changed-groups.sh`) would only add runtime for
  no correctness gain.
- Changed set = uncommitted .go changes PLUS packages changed by commits since
  the last *green* verify. The green baseline is the `git_sha` recorded in
  `tmp/ze-verify.status`, used only when that run's `exit=0` and the SHA is a
  reachable commit. This catches committed-but-unverified packages precisely,
  with minimal extra runtime (only the newly committed packages).
- Fall back to working-tree-only when there is no trusted green baseline
  (missing status, `exit!=0`, or unreachable SHA). Degrades to the historical
  behaviour, never worse; chosen over diffing against `origin/main`, which on a
  branch tens of commits ahead would make scoped verify nearly as slow as full.
- Extracted the logic into one shared `scripts/dev/changed-pkgs.sh` consumed by
  both scoped Makefile targets, instead of editing two duplicated inline
  pipelines, so the rule lives in one tested place.
- Filter to directories that still exist, so a committed *deletion* does not emit
  a `./pkg` path that `go test`/`golangci-lint` would choke on.

## Consequences

- After committing a package and before the next green verify, `make
  ze-verify-changed` re-tests that package; a red commit can no longer hide
  behind a clean working tree.
- `tmp/ze-verify.status` is now an input to the scoped changed set, not just a
  rerun-skip cache. `verify-status.sh write 0` is what advances the baseline.
- `scripts/dev/changed-pkgs.sh` is covered by `scripts/dev/changed_pkgs_test.go`
  (working-tree, untracked, committed-since-green, clean-at-baseline,
  failed-baseline, invalid-SHA, deleted-package). It runs under `ze-unit-test`
  like the other `scripts/dev` tests.

## Gotchas

- `changed-groups.sh` (full-verify `-race` pass) was intentionally left
  working-tree-only; do not assume the two changed-set computations match.
- The baseline is "last green", not "HEAD". If verify has never passed since the
  status file was created with `exit!=0`, the committed-since term is silent by
  design.
- `git diff <sha> HEAD` includes deletions; the existing-directory filter is
  load-bearing, not cosmetic.
- gopls/golangci modernize flags `range strings.Split(...)`; use
  `strings.SplitSeq` (go.mod is >= 1.24).

## Files

None recorded.
