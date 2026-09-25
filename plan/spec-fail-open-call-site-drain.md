# Spec: fail-open-call-site-drain

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Updated | 2026-08-14 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Prevent a failed test-harness command from becoming a passing assertion over
missing output. Before scheduling a drain, determine which instances of the
original defect survive in the native harness and assign each one to its current
owner. The old count of 171 sites across 67 files is not a current worklist.

`docs/architecture/testing/interop.md` places assertions in native checkers
under `internal/le/interoplab/`. At the command boundary, `Docker.Exec` and
`Docker.command` in `internal/le/interoplab/docker.go` return an error for a
nonzero exit and retain output in `CommandResult`. This removes the old
empty-string-only return contract; it does not prove that every caller checks
the error.

The resumed inventory must read the current callers and distinguish propagated
errors, deliberately accepted failures with a reason, and errors discarded
before an assertion. Each surviving instance must either test the failure or
carry a justified opt-out that the current test surface can verify. Each batch
owes failure-path evidence from the scenario it changes. A deleted Python call
site is retired population, not proof that its replacement is correct.

The original shrink-only guard and
`test/health/docker-exec-baseline.json` were retired with the Python harness.
`internal/le/doc/wiring/delegate.go` now dispatches native documentation
checks and does not derive a fail-open call graph. There is no current ratchet
established by this record, so the earlier claim that growth is capped is
withdrawn. Any surviving drain must identify its current guard and regression
surface before relying on one; do not restore the old baseline or Python helper.

Compare the inventory with `plan/spec-harness-fail-open-guard-backlog.md`
before assigning repairs. Its August survey excludes `docker_rm` teardown
contracts from this drain and holds separate guard and assertion questions.
That boundary remains unless the owner changes it. No native call-site sweep
or implementation is claimed by this reconciliation.

## Historical Population

On 2026-08-14, `docker_exec_quiet` in `test/interop/interop.py` returned `""`
on a nonzero exit. The original drain covered 171 unchecked reads across 67
files, with `# fail-open-ok: <reason>` as the justified exception form.
The Python helper, baseline and `./le test functional docker-exec-check` recipe
are retained here as provenance for that population, not current commands.

## Provenance

Homed here 2026-08-14 when `spec-fixit-test-harness-fail-open-guards` closed.
It was the one row of that spec's deferral shard with no destination: the guard
(its guard 4) is what refuses the next site, and draining the existing ones was
never in its scope. Thomas has not commissioned this; it is written down so the
row has a home rather than to schedule the work.
