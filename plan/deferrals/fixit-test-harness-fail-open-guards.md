# Deferrals: fixit-test-harness-fail-open-guards

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-09 | spec-fixit-test-harness-fail-open-guards (guard 4) | Changed-file routing for `ze-docker-exec-check`: `is_docker_exec_source` in `scripts/dev/verify_wiring_docs.py` plus `DockerExecRoutingTest` in its sibling test. Both are written and green; neither is committed | Another session's `plan/learned` to `plan/journal` refactor interleaved 12 lines into `verify_wiring_docs.py` between implementation and commit, so committing the file would carry their half-finished work into HEAD. The floor is enforced meanwhile by `TestRepoRatchet`, which `scripts/dev/python_tests_test.go` runs under `make ze-unit-test`, so this is a changed-file optimisation rather than the guard | `plan/spec-fixit-test-harness-fail-open-guards.md` (this spec stays open for it) | deferred |
| 2026-08-09 | spec-fixit-test-harness-fail-open-guards (guard 4) | The `ze-docker-exec-check` row in the repo-maintenance gate table, and its `ai/INDEX.md` entry | The point file is a clean one-line addition, but `ai/rules/repo-maintenance.md` is GENERATED and cannot be regenerated without the same session's renamed points; `ai/INDEX.md` carries 67 of their lines against 1 of mine | `plan/spec-fixit-test-harness-fail-open-guards.md` | deferred |
| 2026-08-09 | spec-fixit-test-harness-fail-open-guards (guard 4) | 171 unchecked fail-open call sites across 66 files | The ratchet refuses the next one, which is what guard 4 is for. Driving the existing 171 to zero is mechanical work across interop scenarios that mostly cannot run without Docker, and it does not belong in the same change as the guard | unassigned: needs its own spec, and Thomas has not commissioned one | deferred |
