# Deferrals: fixit-test-harness-fail-open-guards

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

All three rows reached a terminal status on 2026-08-14, when
`spec-fixit-test-harness-fail-open-guards` closed. The evidence for each is in
that spec's Deferrals Resolved table, preserved in git history by the commit
that files this file's terminal statuses. The Status cell carries the vocabulary
word alone, because `DEFERRAL_TERMINAL_STATUSES` (`scripts/dev/commit_helper.py`)
matches the whole cell and reads a prose status as live. That is also why the
removal of this file is a LATER commit: `deferral_shard_removal_problems` reads
the shard at HEAD, so the terminal statuses must land first.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | spec-fixit-test-harness-fail-open-guards (guard 4) | Changed-file routing for `ze-docker-exec-check`: `is_docker_exec_source` in `scripts/dev/verify_wiring_docs.py` plus `DockerExecRoutingTest` in its sibling test. Both are written and green; neither is committed | Another session's `plan/learned` to `plan/journal` refactor interleaved 12 lines into `verify_wiring_docs.py` between implementation and commit, so committing the file would carry their half-finished work into HEAD. The floor is enforced meanwhile by `TestRepoRatchet`, which `scripts/dev/python_tests_test.go` runs under `make ze-unit-test`, so this is a changed-file optimisation rather than the guard | `spec-fixit-test-harness-fail-open-guards` (this spec stays open for it) | done |
| 2026-08-09 | spec-fixit-test-harness-fail-open-guards (guard 4) | The `ze-docker-exec-check` row in the repo-maintenance gate table, and its `ai/INDEX.md` entry | The point file is a clean one-line addition, but `ai/rules/repo-maintenance.md` is GENERATED and cannot be regenerated without the same session's renamed points; `ai/INDEX.md` carries 67 of their lines against 1 of mine | `spec-fixit-test-harness-fail-open-guards` | done |
| 2026-08-09 | spec-fixit-test-harness-fail-open-guards (guard 4) | 171 unchecked fail-open call sites across 67 files | The ratchet refuses the next one, which is what guard 4 is for. Driving the existing 171 to zero is mechanical work across interop scenarios that mostly cannot run without Docker, and it does not belong in the same change as the guard | `plan/future/spec-fail-open-call-site-drain.md` | resolved |
