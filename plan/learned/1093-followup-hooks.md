# 1093 -- followup-hooks

## Context

Three agent-guard hooks were dead or broken. `c_format_alloc` (the BGP
format-file append-idiom guard in `pretool-writeedit.py`) `return`ed early on a
dead code path. `validate-spec.sh` crashed under `set -e` on any spec whose
Wiring Test used ASCII `->` arrows (it matched only the Unicode arrow), so an
empty `grep` pipeline exited 1 and aborted the script before its verdict,
swallowing every queued error as a silent non-blocking exit 1. And four
commit-time gates (deferral-unassigned, deferral-in-diff, wiring-at-commit,
doc-drift) plus a never-ported spec-audit check gated on the literal `git
commit` string, which the sanctioned `bash tmp/commit-<SID>.sh` path never sends
and `check_destructive_git` blocks when it does. The goal was to make all three
live, tested, and discoverable without weakening any existing gate.

## Decisions

- Re-homed the commit-time gates into `commit_helper.py` creation-time gates
  (over widening the pretool-bash substring match): the helper already hosts
  repo-state gates and knows the exact add/remove set, and the sanctioned path
  never carries the `git commit` string for pretool-bash to match. The
  prospective diff is computed in a throwaway `GIT_INDEX_FILE` so the real
  staging area is untouched and brand-new files are still captured.
- Enabled `c_format_alloc` as blocking with a comment exemption copied from
  `c_sprintf_new` (over a bare early-`return` removal): its incremental value
  over `c_sprintf_new` is the `strings.Join/Builder/NewReplacer/ReplaceAll`
  bans. Dropped the deleted `bgp/attribute/text.go`, added `bgp/format/json.go`.
- Kept `validate-spec.sh` exit-2 blocking (over warn-only), gated on the AC-4
  survey: the arrow fix introduced zero false positives, and the newly-surfaced
  blocks are genuine structural errors the crash had been hiding.
- Ported spec-audit keyed to the live per-session marker (`spec-session.sh
  current`) and to the commit's own learned summary, over resurrecting the
  removed `tmp/session/selected-spec` substrate. It fires ONLY on the claiming
  session's closure commit, which removes the historic umbrella-spec false
  positives.
- Put the format-alloc and commit-gate tests in a new
  `scripts/dev/hook-fixture-check.py` (over the `hook-parity-check.py` WE
  corpus): the parity harness measures whole-dispatcher exit codes in a non-git
  dir, where `c_pre_write_go`/`c_require_design_ref` return 2 for any
  `internal/*.go` and the commit gates have no git repo, so it can never isolate
  these checks.

## Consequences

- `make ze-hook-test` runs both `hook-parity-check.py` (golden exit codes) and
  `hook-fixture-check.py` (behavioural fixtures); listed in `repo-maintenance.md`.
- The deferral and spec-audit gates are now LIVE on every commit-script
  creation: an open unassigned deferral, a prose deferral without a
  `plan/deferrals.md` entry, or a closure commit with an unfilled `Pre-Commit
  Verification` section now block. None of this was enforced before.
- `deferral_in_diff` matches only PROSE: quoted and backticked spans are blanked
  before matching (`_deferral_prose`), so the pattern list, its rule docs, and
  its own fixtures do not self-trip; a phrase in bare markdown or a bare comment
  is still caught.

## Gotchas

- The `hook-parity-check.py` golden was macOS-blessed. On Linux
  `tempfile.mkdtemp()` returns a `/tmp` path, which trips `c_system_tmp_we` (and
  a repo-tree path pulls fixture `.go` into the module, so posttool lint runs),
  forcing exit 2 on every sub-2 case. Ten cases mismatched against HEAD's OWN
  hooks -- pure platform drift, not a behaviour change. Fixed by anchoring
  fixture dirs under `~/.cache`; blessed nothing. Lesson: when a golden diff
  appears, run the check against `HEAD`'s code first to separate your change from
  environment drift.
- Enabling `c_format_alloc` verbatim would block full-file writes of
  `bgp/format/text.go`, whose header comment names `strings.Builder` and
  `strings.Join`. The comment exemption is load-bearing, not cosmetic.
- A phrase-detector on the gated commit path detects the vocabulary it is
  defined with: committing the gate, or editing its rule doc or fixtures,
  self-tripped until prose-only matching (`_deferral_prose`) was added.
- `validate-spec.sh` crashed via `set -e` on a bare `VAR=$(pipeline)` whose
  `grep` selected nothing (pipeline exit 1). `|| true` on the assignment is the
  fix; audit every `VAR=$(... | grep ...)` for this shape.

## Files

- `.claude/hooks/pretool-writeedit.py` -- enabled `c_format_alloc`
- `.claude/hooks/validate-spec.sh` -- arrow + `set -e` fix
- `.claude/hooks/pretool-bash.py` -- removed four re-homed commit gates
- `scripts/dev/commit_helper.py` -- commit-time gates
- `scripts/dev/hook-parity-check.py` -- portable fixture dir
- `scripts/dev/hook-fixture-check.py` -- NEW behavioural fixtures
- `Makefile` -- `ze-hook-test` target
- `ai/rules/repo-maintenance.md` -- corrected stale rows
